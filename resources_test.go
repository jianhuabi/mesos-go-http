package mesos_test

import (
	"fmt"
	"github.com/ondrej-smola/mesos-go-http"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resources", func() {

	It("PrecisionRounding", func() {
		var (
			cpu = resources(resource(name("cpus"), valueScalar(1.5015)))
			r1  = cpu.Plus(cpu...).Plus(cpu...).Minus(cpu...).Minus(cpu...)
		)
		if !cpu.Equivalent(r1) {
			Fail(fmt.Sprintf("expected %v instead of %v", cpu, r1))
		}
		actual, ok := r1.CPUs()
		Expect(ok).To(BeTrue())
		Expect(actual).To(Equal(1.502))
	})

	It("PrecisionLost", func() {
		var (
			cpu = resources(resource(name("cpus"), valueScalar(1.5011)))
			r1  = cpu.Plus(cpu...).Plus(cpu...).Minus(cpu...).Minus(cpu...)
		)

		if !cpu.Equivalent(r1) {
			Fail(fmt.Sprintf("expected %v instead of %v", cpu, r1))
		}
		actual, ok := r1.CPUs()
		Expect(ok).To(BeTrue())
		Expect(actual).To(Equal(1.501))
	})

	It("PrecisionManyConsecutiveOps", func() {

		var (
			start     = resources(resource(name("cpus"), valueScalar(1.001)))
			increment = start.Clone()
			current   = start.Clone()
		)
		for i := 0; i < 100000; i++ {
			current.Add(increment...)
		}
		for i := 0; i < 100000; i++ {
			current.Subtract(increment...)
		}
		Expect(start).To(Equal(current))
	})

	It("PrecisionManyOps", func() {
		var (
			start   = resources(resource(name("cpus"), valueScalar(1.001)))
			current = start.Clone()
			next    mesos.Resources
		)
		for i := 0; i < 2500; i++ {
			next = current.Plus(current...).Plus(current...).Minus(current...).Minus(current...)
			actual, ok := next.CPUs()

			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(1.001))
			Expect(current).To(Equal(next))
			Expect(start).To(Equal(next))
		}
	})

	It("PrecisionSimple", func() {
		var (
			cpu  = resources(resource(name("cpus"), valueScalar(1.001)))
			zero = mesos.Resources{resource(name("cpus"), valueScalar(0))} // don't validate
		)
		actual, ok := cpu.CPUs()
		Expect(ok).To(BeTrue())
		Expect(actual).To(Equal(1.001))

		x := cpu.Plus(zero...)
		Expect(x).To(Equal(cpu))
		y := cpu.Minus(zero...)
		Expect(y).To(Equal(cpu))
	})

	It("Types", func() {
		rs := resources(
			resource(name("cpus"), valueScalar(2), role("role1")),
			resource(name("cpus"), valueScalar(4)),
			resource(name("ports"), valueRange(span(1, 10)), role("role1")),
			resource(name("ports"), valueRange(span(11, 20))),
		)
		types := rs.Types()
		expected := map[string]mesos.Value_Type{
			"cpus":  mesos.Value_SCALAR,
			"ports": mesos.Value_RANGES,
		}
		Expect(expected).To(Equal(types))
	})

	It("Names", func() {
		rs := resources(
			resource(name("cpus"), valueScalar(2), role("role1")),
			resource(name("cpus"), valueScalar(4)),
			resource(name("mem"), valueScalar(10), role("role1")),
			resource(name("mem"), valueScalar(10)),
		)
		names := rs.Names()
		expected := []string{"cpus", "mem"}

		Expect(expected).To(ConsistOf(names))
	})

	It("RevocableResources", func() {
		rs := mesos.Resources{
			resource(name("cpus"), valueScalar(1), role("*"), revocable()),
			resource(name("cpus"), valueScalar(1), role("*")),
		}
		for i, tc := range []struct {
			r1, wants mesos.Resources
		}{
			{resources(rs[0]), resources(rs[0])},
			{resources(rs[1]), resources()},
			{resources(rs[0], rs[1]), resources(rs[0])},
		} {
			x := mesos.RevocableResources.Select(tc.r1)
			if !tc.wants.Equivalent(x) {
				Fail(fmt.Sprintf("test case %d failed: expected %v instead of %v", i, tc.wants, x))
			}
		}
	})

	It("PersistentVolumes", func() {
		var (
			rs = resources(
				resource(name("cpus"), valueScalar(1)),
				resource(name("mem"), valueScalar(512)),
				resource(name("disk"), valueScalar(1000)),
			)
			disk = mesos.Resources{
				resource(name("disk"), valueScalar(10), role("role1"), disk("1", "path")),
				resource(name("disk"), valueScalar(20), role("role2"), disk("", "")),
			}
		)
		rs.Add(disk...)
		pv := mesos.PersistentVolumes.Select(rs)

		if !resources(disk[0]).Equivalent(pv) {
			Fail(fmt.Sprintf("expected %v instead of %v", resources(disk[0]), pv))
		}
	})

	It("Validation", func() {
		// don't use resources(...) because that implicitly validates and skips invalid resources
		rs := mesos.Resources{
			resource(name("cpus"), valueScalar(2), role("*"), disk("1", "path")),
		}
		err := rs.Validate()
		if resourceErr, ok := err.(*mesos.ResourceError); !ok || resourceErr.Type() != mesos.ResourceErrorTypeIllegalDisk {
			Fail("expected error because cpu resources can't contain disk info")
		}

		err = mesos.Resources{resource(name("disk"), valueScalar(10), role("role"), disk("1", "path"))}.Validate()
		if err != nil {
			Fail(fmt.Sprintf("unexpected error: %+v", err))
		}

		err = mesos.Resources{resource(name("disk"), valueScalar(10), role("role"), disk("", "path"))}.Validate()
		if err != nil {
			Fail(fmt.Sprintf("unexpected error: %+v", err))
		}

		// reserved resources

		// unreserved:
		err = mesos.Resources{resource(name("cpus"), valueScalar(8), role("*"))}.Validate()
		if err != nil {
			Fail(fmt.Sprintf("unexpected error validating unreserved resource: %+v", err))
		}

		// statically role reserved:
		err = mesos.Resources{resource(name("cpus"), valueScalar(8), role("role"))}.Validate()
		if err != nil {
			Fail(fmt.Sprintf("unexpected error validating statically role reserved resource: %+v", err))
		}

		// dynamically role reserved:
		err = mesos.Resources{resource(name("cpus"), valueScalar(8), role("role"), reservation(reservedBy("principal2")))}.Validate()
		if err != nil {
			Fail(fmt.Sprintf("unexpected error validating dynamically role reserved resource: %+v", err))
		}

		// invalid
		err = mesos.Resources{resource(name("cpus"), valueScalar(8), role("*"), reservation(reservedBy("principal1")))}.Validate()
		if err == nil {
			Fail("expected error for invalid reserved resource")
		}
	})

	It("Find", func() {
		for i, tc := range []struct {
			r1, targets, wants mesos.Resources
		}{
			{nil, nil, nil},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(2), role("role1")),
					resource(name("mem"), valueScalar(10), role("role1")),
					resource(name("cpus"), valueScalar(4), role("*")),
					resource(name("mem"), valueScalar(20), role("*")),
				),
				targets: resources(
					resource(name("cpus"), valueScalar(3), role("role1")),
					resource(name("mem"), valueScalar(15), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(2), role("role1")),
					resource(name("mem"), valueScalar(10), role("role1")),
					resource(name("cpus"), valueScalar(1), role("*")),
					resource(name("mem"), valueScalar(5), role("*")),
				),
			},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("mem"), valueScalar(5), role("role1")),
					resource(name("cpus"), valueScalar(2), role("role2")),
					resource(name("mem"), valueScalar(8), role("role2")),
					resource(name("cpus"), valueScalar(1), role("*")),
					resource(name("mem"), valueScalar(7), role("*")),
				),
				targets: resources(
					resource(name("cpus"), valueScalar(3), role("role1")),
					resource(name("mem"), valueScalar(15), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("mem"), valueScalar(5), role("role1")),
					resource(name("cpus"), valueScalar(1), role("*")),
					resource(name("mem"), valueScalar(7), role("*")),
					resource(name("cpus"), valueScalar(1), role("role2")),
					resource(name("mem"), valueScalar(3), role("role2")),
				),
			},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(5), role("role1")),
					resource(name("mem"), valueScalar(5), role("role1")),
					resource(name("cpus"), valueScalar(5), role("*")),
					resource(name("mem"), valueScalar(5), role("*")),
				),
				targets: resources(
					resource(name("cpus"), valueScalar(6)),
					resource(name("mem"), valueScalar(6)),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(5), role("*")),
					resource(name("mem"), valueScalar(5), role("*")),
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("mem"), valueScalar(1), role("role1")),
				),
			},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("mem"), valueScalar(1), role("role1")),
				),
				targets: resources(
					resource(name("cpus"), valueScalar(2), role("role1")),
					resource(name("mem"), valueScalar(2), role("role1")),
				),
				wants: nil,
			},
		} {
			r := tc.r1.Find(tc.targets)
			if !r.Equivalent(tc.wants) {
				Fail(fmt.Sprintf("test case %d failed: expected %+v instead of %+v", i, tc.wants, r))
			}
		}
	})

	It("Flatten", func() {
		for i, tc := range []struct {
			r1, wants mesos.Resources
		}{
			{nil, nil},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("cpus"), valueScalar(2), role("role2")),
					resource(name("mem"), valueScalar(5), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(3)),
					resource(name("mem"), valueScalar(5)),
				),
			},
			{
				r1: resources(
					resource(name("cpus"), valueScalar(3), role("role1")),
					resource(name("mem"), valueScalar(15), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(3), role("*")),
					resource(name("mem"), valueScalar(15), role("*")),
				),
			},
		} {
			r := tc.r1.Flatten()
			if !r.Equivalent(tc.wants) {
				Fail(fmt.Sprintf("test case %d failed: expected %+v instead of %+v", i, tc.wants, r))
			}
		}
	})

	It("Equivalent", func() {
		disks := mesos.Resources{
			resource(name("disk"), valueScalar(10), role("*"), disk("", "")),
			resource(name("disk"), valueScalar(10), role("*"), disk("", "path1")),
			resource(name("disk"), valueScalar(10), role("*"), disk("", "path2")),
			resource(name("disk"), valueScalar(10), role("role"), disk("", "path2")),
			resource(name("disk"), valueScalar(10), role("role"), disk("1", "path1")),
			resource(name("disk"), valueScalar(10), role("role"), disk("1", "path2")),
			resource(name("disk"), valueScalar(10), role("role"), disk("2", "path2")),
		}
		for i, tc := range []struct {
			r1, r2 mesos.Resources
			wants  bool
		}{
			{r1: nil, r2: nil, wants: true},
			{ // 1
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				wants: true,
			},
			{ // 2
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("role1")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("role2")),
				),
				wants: false,
			},
			{ // 3
				r1:    resources(resource(name("ports"), valueRange(span(20, 40)), role("*"))),
				r2:    resources(resource(name("ports"), valueRange(span(20, 30), span(31, 39), span(40, 40)), role("*"))),
				wants: true,
			},
			{ // 4
				r1:    resources(resource(name("disks"), valueSet("sda1"), role("*"))),
				r2:    resources(resource(name("disks"), valueSet("sda1"), role("*"))),
				wants: true,
			},
			{ // 5
				r1:    resources(resource(name("disks"), valueSet("sda1"), role("*"))),
				r2:    resources(resource(name("disks"), valueSet("sda2"), role("*"))),
				wants: false,
			},
			{resources(disks[0]), resources(disks[1]), true},  // 6
			{resources(disks[1]), resources(disks[2]), true},  // 7
			{resources(disks[4]), resources(disks[5]), true},  // 8
			{resources(disks[5]), resources(disks[6]), false}, // 9
			{resources(disks[3]), resources(disks[6]), false}, // 10
			{ // 11
				r1:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				r2:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				wants: true,
			},
			{ // 12
				r1:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				r2:    resources(resource(name("cpus"), valueScalar(1), role("*"))),
				wants: false,
			},
		} {
			actual := tc.r1.Equivalent(tc.r2)
			if !tc.wants == actual {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual))
			}
		}

		possiblyReserved := mesos.Resources{
			// unreserved
			resource(name("cpus"), valueScalar(8), role("*")),
			// statically role reserved
			resource(name("cpus"), valueScalar(8), role("role1")),
			resource(name("cpus"), valueScalar(8), role("role2")),
			// dynamically role reserved:
			resource(name("cpus"), valueScalar(8), role("role1"), reservation(reservedBy("principal1"))),
			resource(name("cpus"), valueScalar(8), role("role2"), reservation(reservedBy("principal2"))),
		}
		for i := 0; i < len(possiblyReserved); i++ {
			for j := 0; j < len(possiblyReserved); j++ {
				if i == j {
					continue
				}
				if resources(possiblyReserved[i]).Equivalent(resources(possiblyReserved[j])) {
					Fail(fmt.Sprintf("unexpected equivalence between %v and %v", possiblyReserved[i], possiblyReserved[j]))
				}
			}
		}
	})

	It("ContainsAll", func() {
		var (
			ports1 = resources(resource(name("ports"), valueRange(span(2, 2), span(4, 5)), role("*")))
			ports2 = resources(resource(name("ports"), valueRange(span(1, 10)), role("*")))
			ports3 = resources(resource(name("ports"), valueRange(span(2, 3)), role("*")))
			ports4 = resources(resource(name("ports"), valueRange(span(1, 2), span(4, 6)), role("*")))
			ports5 = resources(resource(name("ports"), valueRange(span(1, 4), span(5, 5)), role("*")))

			disks1 = resources(resource(name("disks"), valueSet("sda1", "sda2"), role("*")))
			disks2 = resources(resource(name("disks"), valueSet("sda1", "sda3", "sda4", "sda2"), role("*")))

			disks = mesos.Resources{
				resource(name("disk"), valueScalar(10), role("role"), disk("1", "path")),
				resource(name("disk"), valueScalar(10), role("role"), disk("2", "path")),
				resource(name("disk"), valueScalar(20), role("role"), disk("1", "path")),
				resource(name("disk"), valueScalar(20), role("role"), disk("", "path")),
				resource(name("disk"), valueScalar(20), role("role"), disk("2", "path")),
			}
			summedDisks  = resources(disks[0]).Plus(disks[1])
			summedDisks2 = resources(disks[0]).Plus(disks[4])

			revocables = mesos.Resources{
				resource(name("cpus"), valueScalar(1), role("*"), revocable()),
				resource(name("cpus"), valueScalar(1), role("*")),
				resource(name("cpus"), valueScalar(2), role("*")),
				resource(name("cpus"), valueScalar(2), role("*"), revocable()),
			}
			summedRevocables  = resources(revocables[0]).Plus(revocables[1])
			summedRevocables2 = resources(revocables[0]).Plus(revocables[0])

			possiblyReserved = mesos.Resources{
				resource(name("cpus"), valueScalar(8), role("role")),
				resource(name("cpus"), valueScalar(12), role("role"), reservation(reservedBy("principal"))),
			}
			sumPossiblyReserved = resources(possiblyReserved...)
		)
		for i, tc := range []struct {
			r1, r2 mesos.Resources
			wants  bool
		}{
			// test case 0
			{r1: nil, r2: nil, wants: true},
			// test case 1
			{
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				wants: true,
			},
			// test case 2
			{
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("role1")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("role2")),
				),
				wants: false,
			},
			// test case 3
			{
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(3072), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				wants: false,
			},
			// test case 4
			{
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(3072), role("*")),
				),
				wants: true,
			},
			// test case 5
			{ports2, ports1, true},
			// test case 6
			{ports1, ports2, false},
			// test case 7
			{ports3, ports1, false},
			// test case 8
			{ports1, ports3, false},
			// test case 9
			{ports2, ports3, true},
			// test case 10
			{ports3, ports2, false},
			// test case 11
			{ports4, ports1, true},
			// test case 12
			{ports2, ports4, true},
			// test case 13
			{ports5, ports1, true},
			// test case 14
			{ports1, ports5, false},
			// test case 15
			{disks1, disks2, false},
			// test case 16
			{disks2, disks1, true},
			{r1: summedDisks, r2: resources(disks[0]), wants: true},
			{r1: summedDisks, r2: resources(disks[1]), wants: true},
			{r1: summedDisks, r2: resources(disks[2]), wants: false},
			{r1: summedDisks, r2: resources(disks[3]), wants: false},
			{r1: resources(disks[0]), r2: summedDisks, wants: false},
			{r1: resources(disks[1]), r2: summedDisks, wants: false},
			{r1: summedDisks2, r2: resources(disks[0]), wants: true},
			{r1: summedDisks2, r2: resources(disks[4]), wants: true},
			{r1: summedRevocables, r2: resources(revocables[0]), wants: true},
			{r1: summedRevocables, r2: resources(revocables[1]), wants: true},
			{r1: summedRevocables, r2: resources(revocables[2]), wants: false},
			{r1: summedRevocables, r2: resources(revocables[3]), wants: false},
			{r1: resources(revocables[0]), r2: summedRevocables2, wants: false},
			{r1: summedRevocables2, r2: resources(revocables[0]), wants: true},
			{r1: summedRevocables2, r2: summedRevocables2, wants: true},
			{r1: resources(possiblyReserved[0]), r2: sumPossiblyReserved, wants: false},
			{r1: resources(possiblyReserved[1]), r2: sumPossiblyReserved, wants: false},
			{r1: sumPossiblyReserved, r2: sumPossiblyReserved, wants: true},
		} {
			actual := tc.r1.ContainsAll(tc.r2)
			if !tc.wants == actual {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual))
			}
		}

	})

	It("IsEmpty", func() {
		for i, tc := range []struct {
			r     mesos.Resource
			wants bool
		}{
			{resource(), true},
			{resource(valueScalar(0)), true},
			{resource(valueSet()), true},
			{resource(valueSet([]string{}...)), true},
			{resource(valueSet()), true},
			{resource(valueSet("")), false},
			{resource(valueRange()), true},
			{resource(valueRange(span(0, 0))), false},
		} {
			actual := tc.r.IsEmpty()
			if !tc.wants == actual {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual))
			}
		}
	})
	It("Minus", func() {
		disks := mesos.Resources{
			resource(name("disk"), valueScalar(10), role("role"), disk("", "path")),
			resource(name("disk"), valueScalar(10), role("role"), disk("", "")),
			resource(name("disk"), valueScalar(10), role("role"), disk("1", "path")),
			resource(name("disk"), valueScalar(10), role("role"), disk("2", "path")),
			resource(name("disk"), valueScalar(10), role("role"), disk("2", "path2")),
		}
		for i, tc := range []struct {
			r1, r2      mesos.Resources
			wants       mesos.Resources
			wantsCPU    float64
			wantsMemory uint64
		}{
			{r1: nil, r2: nil, wants: nil},
			{r1: resources(), r2: resources(), wants: resources()},
			// simple scalars, same roles for everything
			{
				r1: resources(
					resource(name("cpus"), valueScalar(50), role("*")),
					resource(name("mem"), valueScalar(4096), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(0.5), role("*")),
					resource(name("mem"), valueScalar(1024), role("*")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(49.5), role("*")),
					resource(name("mem"), valueScalar(3072), role("*")),
				),
				wantsCPU:    49.5,
				wantsMemory: 3072,
			},
			// multi-role, scalar subtraction
			{
				r1: resources(
					resource(name("cpus"), valueScalar(5), role("role1")),
					resource(name("cpus"), valueScalar(3), role("role2")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(4), role("role1")),
					resource(name("cpus"), valueScalar(3), role("role2")),
				),
				wantsCPU: 7,
			},
			// simple ranges, same roles, lower-edge overlap
			{
				r1: resources(
					resource(name("ports"), valueRange(span(20000, 40000)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(10000, 20000), span(30000, 50000)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(20001, 29999)), role("*")),
				),
			},
			// simple ranges, same roles, single port/lower-edge
			{
				r1: resources(
					resource(name("ports"), valueRange(span(50000, 60000)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(50000, 50000)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(50001, 60000)), role("*")),
				),
			},
			// simple ranges, same roles, multi port/lower-edge
			{
				r1: resources(
					resource(name("ports"), valueRange(span(50000, 60000)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(50000, 50001)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(50002, 60000)), role("*")),
				),
			},
			// simple ranges, same roles, identical overlap
			{
				r1: resources(
					resource(name("ports"), valueRange(span(50000, 60000)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(50000, 60000)), role("*")),
				),
				wants: resources(),
			},
			// multiple ranges, same roles, swiss cheese
			{
				r1: resources(
					resource(name("ports"), valueRange(span(1, 10), span(20, 30), span(40, 50)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(2, 9), span(15, 45), span(48, 50)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(1, 1), span(10, 10), span(46, 47)), role("*")),
				),
			},
			// multiple ranges, same roles, no overlap
			{
				r1: resources(
					resource(name("ports"), valueRange(span(1, 10)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(11, 20)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(1, 10)), role("*")),
				),
			},
			// simple set, same roles
			{
				r1: resources(
					resource(name("disks"), valueSet("sda1", "sda2", "sda3", "sda4"), role("*")),
				),
				r2: resources(
					resource(name("disks"), valueSet("sda2", "sda3", "sda4"), role("*")),
				),
				wants: resources(
					resource(name("disks"), valueSet("sda1"), role("*")),
				),
			},
			{r1: resources(disks[0]), r2: resources(disks[1]), wants: resources()},
			{r1: resources(disks[2]), r2: resources(disks[3]), wants: resources(disks[2])},
			{r1: resources(disks[2]), r2: resources(disks[2]), wants: resources()},
			{r1: resources(disks[3]), r2: resources(disks[4]), wants: resources()},
			// revocables
			{
				r1:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				r2:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				wants: resources(),
			},
			{ // revocable - non-revocable is a noop
				r1:       resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				r2:       resources(resource(name("cpus"), valueScalar(1), role("*"))),
				wants:    resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				wantsCPU: 1,
			},
			// reserved
			{
				r1: resources(
					resource(name("cpus"), valueScalar(8), role("role")),
					resource(name("cpus"), valueScalar(8), role("role"), reservation(reservedBy("principal"))),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(2), role("role")),
					resource(name("cpus"), valueScalar(4), role("role"), reservation(reservedBy("principal"))),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(6), role("role")),
					resource(name("cpus"), valueScalar(4), role("role"), reservation(reservedBy("principal"))),
				),
				wantsCPU: 10,
			},
		} {
			backup := tc.r1.Clone()

			// Minus preserves the left operand
			actual := tc.r1.Minus(tc.r2...)
			if !tc.wants.Equivalent(actual) {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual))
			}
			if !backup.Equivalent(tc.r1) {
				Fail(fmt.Sprintf("test case %d failed: backup (%v) != r1 (%v)", i, backup, tc.r1))
			}

			// SubtractAll mutates the left operand
			tc.r1.Subtract(tc.r2...)
			if !tc.wants.Equivalent(tc.r1) {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != r1 (%v)", i, tc.wants, tc.r1))
			}

			cpus, ok := tc.r1.CPUs()
			if !ok && tc.wantsCPU > 0 {
				Fail(fmt.Sprintf("test case %d failed: failed to obtain total CPU resources", i))
			} else if cpus != tc.wantsCPU {
				Fail(fmt.Sprintf("test case %d failed: wants cpu (%v) != r1 cpu (%v)", i, tc.wantsCPU, cpus))
			}

			mem, ok := tc.r1.Memory()
			if !ok && tc.wantsMemory > 0 {
				Fail(fmt.Sprintf("test case %d failed: failed to obtain total memory resources", i))
			} else if mem != tc.wantsMemory {
				Fail(fmt.Sprintf("test case %d failed: wants mem (%v) != r1 mem (%v)", i, tc.wantsMemory, mem))
			}

			tc.r1.Subtract(tc.r1...)
			if len(tc.r1) > 0 {
				Fail(fmt.Sprintf("test case %d failed: r1 is not empty (%v)", i, tc.r1))
			}
		}

	})
	It("Plus", func() {
		disks := mesos.Resources{
			resource(name("disk"), valueScalar(10), role("role"), disk("", "path")),
			resource(name("disk"), valueScalar(10), role("role"), disk("", "")),
			resource(name("disk"), valueScalar(20), role("role"), disk("", "path")),
		}
		for i, tc := range []struct {
			r1, r2      mesos.Resources
			wants       mesos.Resources
			wantsCPU    float64
			wantsMemory uint64
		}{
			{r1: resources(disks[0]), r2: resources(disks[1]), wants: resources(disks[2])},
			{r1: nil, r2: nil, wants: nil},
			{r1: resources(), r2: resources(), wants: resources()},
			// simple scalars, same roles for everything
			{
				r1: resources(
					resource(name("cpus"), valueScalar(1), role("*")),
					resource(name("mem"), valueScalar(5), role("*")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(2), role("*")),
					resource(name("mem"), valueScalar(10), role("*")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(3), role("*")),
					resource(name("mem"), valueScalar(15), role("*")),
				),
				wantsCPU:    3,
				wantsMemory: 15,
			},
			// simple scalars, differing roles
			{
				r1: resources(
					resource(name("cpus"), valueScalar(1), role("role1")),
					resource(name("cpus"), valueScalar(3), role("role2")),
				),
				r2: resources(
					resource(name("cpus"), valueScalar(5), role("role1")),
				),
				wants: resources(
					resource(name("cpus"), valueScalar(6), role("role1")),
					resource(name("cpus"), valueScalar(3), role("role2")),
				),
				wantsCPU: 9,
			},
			// ranges addition yields continuous range
			{
				r1: resources(
					resource(name("ports"), valueRange(span(20000, 40000)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(30000, 50000), span(10000, 20000)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(10000, 50000)), role("*")),
				),
			},
			// ranges addition yields a split set of ranges
			{
				r1: resources(
					resource(name("ports"), valueRange(span(1, 10), span(5, 30), span(50, 60)), role("*")),
					resource(name("ports"), valueRange(span(1, 65), span(70, 80)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(1, 65), span(70, 80)), role("*")),
				),
			},
			// ranges addition (composite) yields a continuous range
			{
				r1: resources(
					resource(name("ports"), valueRange(span(1, 2)), role("*")),
					resource(name("ports"), valueRange(span(3, 4)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(7, 8)), role("*")),
					resource(name("ports"), valueRange(span(5, 6)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(1, 8)), role("*")),
				),
			},
			// ranges addition yields a split set of ranges
			{
				r1: resources(
					resource(name("ports"), valueRange(span(1, 4), span(9, 10), span(20, 22), span(26, 30)), role("*")),
				),
				r2: resources(
					resource(name("ports"), valueRange(span(5, 8), span(23, 25)), role("*")),
				),
				wants: resources(
					resource(name("ports"), valueRange(span(1, 10), span(20, 30)), role("*")),
				),
			},
			// set addition
			{
				r1: resources(
					resource(name("disks"), valueSet("sda1", "sda2", "sda3"), role("*")),
				),
				r2: resources(
					resource(name("disks"), valueSet("sda1", "sda2", "sda3", "sda4"), role("*")),
				),
				wants: resources(
					resource(name("disks"), valueSet("sda4", "sda2", "sda1", "sda3"), role("*")),
				),
			},
			// revocables
			{
				r1:       resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				r2:       resources(resource(name("cpus"), valueScalar(1), role("*"), revocable())),
				wants:    resources(resource(name("cpus"), valueScalar(2), role("*"), revocable())),
				wantsCPU: 2,
			},
			// statically reserved
			{
				r1:       resources(resource(name("cpus"), valueScalar(8), role("role"))),
				r2:       resources(resource(name("cpus"), valueScalar(4), role("role"))),
				wants:    resources(resource(name("cpus"), valueScalar(12), role("role"))),
				wantsCPU: 12,
			},
			// dynamically reserved
			{
				r1:       resources(resource(name("cpus"), valueScalar(8), role("role"), reservation(reservedBy("principal")))),
				r2:       resources(resource(name("cpus"), valueScalar(4), role("role"), reservation(reservedBy("principal")))),
				wants:    resources(resource(name("cpus"), valueScalar(12), role("role"), reservation(reservedBy("principal")))),
				wantsCPU: 12,
			},
		} {
			backup := tc.r1.Clone()

			// Plus preserves the left operand
			actual := tc.r1.Plus(tc.r2...)
			if !tc.wants.Equivalent(actual) {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual))
			}
			if !backup.Equivalent(tc.r1) {
				Fail(fmt.Sprintf("test case %d failed: backup (%v) != r1 (%v)", i, backup, tc.r1))
			}

			// Add mutates the left operand
			tc.r1.Add(tc.r2...)
			if !tc.wants.Equivalent(tc.r1) {
				Fail(fmt.Sprintf("test case %d failed: wants (%v) != r1 (%v)", i, tc.wants, tc.r1))
			}

			cpus, ok := tc.r1.CPUs()
			if !ok && tc.wantsCPU > 0 {
				Fail(fmt.Sprintf("test case %d failed: failed to obtain total CPU resources", i))
			} else if cpus != tc.wantsCPU {
				Fail(fmt.Sprintf("test case %d failed: wants cpu (%v) != r1 cpu (%v)", i, tc.wantsCPU, cpus))
			}

			mem, ok := tc.r1.Memory()
			if !ok && tc.wantsMemory > 0 {
				Fail(fmt.Sprintf("test case %d failed: failed to obtain total memory resources", i))
			} else if mem != tc.wantsMemory {
				Fail(fmt.Sprintf("test case %d failed: wants mem (%v) != r1 mem (%v)", i, tc.wantsMemory, mem))
			}
		}
	})
})

// functional resource modifier
type resourceOpt func(*mesos.Resource)

func resource(opt ...resourceOpt) (r mesos.Resource) {
	if len(opt) == 0 {
		return
	}
	for _, f := range opt {
		f(&r)
	}
	return
}

func reservation(ri *mesos.Resource_ReservationInfo) resourceOpt {
	return func(r *mesos.Resource) {
		r.Reservation = ri
	}
}

func disk(persistenceID, containerPath string) resourceOpt {
	return func(r *mesos.Resource) {
		r.Disk = &mesos.Resource_DiskInfo{}
		if containerPath != "" {
			r.Disk.Volume = &mesos.Volume{ContainerPath: containerPath}
		}
		if persistenceID != "" {
			r.Disk.Persistence = &mesos.Resource_DiskInfo_Persistence{ID: persistenceID}
		}
	}
}

func reservedBy(principal string) *mesos.Resource_ReservationInfo {
	result := &mesos.Resource_ReservationInfo{}
	if principal != "" {
		result.Principal = &principal
	}
	return result
}

func name(x string) resourceOpt {
	return func(r *mesos.Resource) {
		r.Name = x
	}
}
func role(x string) resourceOpt {
	return func(r *mesos.Resource) {
		r.Role = &x
	}
}

func revocable() resourceOpt {
	return func(r *mesos.Resource) {
		r.Revocable = &mesos.Resource_RevocableInfo{}
	}
}

func valueScalar(x float64) resourceOpt {
	return func(r *mesos.Resource) {
		r.Type = mesos.Value_SCALAR
		r.Scalar = &mesos.Value_Scalar{Value: x}
	}
}

func valueSet(x ...string) resourceOpt {
	return func(r *mesos.Resource) {
		r.Type = mesos.Value_SET
		r.Set = &mesos.Value_Set{Item: x}
	}
}

type rangeOpt func(*mesos.Ranges)

// "range" is a keyword, so I called this func "span": it naively appends a range to a Ranges collection
func span(bp, ep uint64) rangeOpt {
	return func(rs *mesos.Ranges) {
		*rs = append(*rs, mesos.Value_Range{Begin: bp, End: ep})
	}
}

func valueRange(p ...rangeOpt) resourceOpt {
	return func(r *mesos.Resource) {
		rs := mesos.Ranges(nil)
		for _, f := range p {
			f(&rs)
		}
		r.Type = mesos.Value_RANGES
		r.Ranges = r.Ranges.Add(&mesos.Value_Ranges{Range: rs})
	}
}

func resources(r ...mesos.Resource) (result mesos.Resources) {
	return result.Add(r...)
}
