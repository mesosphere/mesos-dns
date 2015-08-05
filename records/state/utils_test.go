package state

import (
	"testing"
)

func TestYankPorts(t *testing.T) {
	p := "[31328-31328]"

	ports := yankPorts(p)

	if ports[0] != "31328" {
		t.Error("not parsing port")
	}
}

func TestMultipleYankPorts(t *testing.T) {
	p := "[31111-31111, 31113-31113]"

	ports := yankPorts(p)

	if len(ports) != 2 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31111" {
		t.Error("not parsing port")
	}

	if ports[1] != "31113" {
		t.Error("not parsing port")
	}
}

func TestRangePorts(t *testing.T) {
	p := "[31115-31117]"

	ports := yankPorts(p)

	if len(ports) != 3 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31115" {
		t.Error("not parsing port")
	}

	if ports[1] != "31116" {
		t.Error("not parsing port")
	}

	if ports[2] != "31117" {
		t.Error("not parsing port")
	}

}
