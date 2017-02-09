package mesos

import (
	"github.com/pkg/errors"
	"net"
)

type Masters []string

var DEFAULT_MASTERS = Masters{"localhost:5050"}

// list of host:port
func NewMasters(m ...string) Masters {
	return Masters(m)
}

func (m Masters) Empty() bool {
	return len(m) == 0
}

func (m Masters) Valid() error {
	if m.Empty() {
		return errors.New("Masters: empty")
	}

	for i, e := range m {
		if _, _, err := net.SplitHostPort(e); err != nil {
			return errors.Wrapf(err, "Masters[%v]:", i)
		}
	}

	return nil
}

func (masters Masters) MakeFirstMaster(m string) Masters {
	replace := -1
	for i, cm := range masters {
		if cm == m {
			replace = i
			break
		}
	}

	if replace == -1 {
		return Masters(append(masters, m))
	} else {
		masters[0], masters[replace] = masters[replace], masters[0]
		return masters
	}
}
