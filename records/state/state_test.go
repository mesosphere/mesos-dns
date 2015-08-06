package state

import (
	"reflect"
	"testing"
)

func TestResources_Ports(t *testing.T) {
	r := Resources{PortRanges: "[31111-31111, 31115-31117]"}
	want := []string{"31111", "31115", "31116", "31117"}
	if got := r.Ports(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}
