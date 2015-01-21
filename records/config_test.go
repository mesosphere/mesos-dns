package records

import (
	"testing"
)

func TestNonLocalAddies(t *testing.T) {
	nlocal := []string{"127.0.0.1"}
	addies := nonLocalAddies(nlocal)

	for i := 0; i < len(addies); i++ {
		if "127.0.0.1" == addies[i] {
			t.Error("finding a local address")
		}
	}
}
