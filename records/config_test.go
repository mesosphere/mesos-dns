package records

import (
	"testing"
)

func TestNonLocalAddies(t *testing.T) {
	nlocal := localAddies()
	addies := nonLocalAddies(nlocal)
	if len(addies) > 0 {
		t.Error("finding a local address")
	}
}

func TestNewConfigValidates(t *testing.T) {
	c := NewConfig()
	err := validateIPSources(c.IPSources)
	if err != nil {
		t.Error(err)
	}
	err = validateResolvers(c.Resolvers)
	if err != nil {
		t.Error(err)
	}
	err = validateMasters(c.Masters)
	if err != nil {
		t.Error(err)
	}
	err = validateEnabledServices(&c)
	if err == nil {
		t.Error("expected error because no masters and no zk servers are configured by default")
	}
	c.Zk = "foo"
	err = validateEnabledServices(&c)
	if err != nil {
		t.Error(err)
	}
}
