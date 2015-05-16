package records

import (
	"testing"
)

func TestValidateResolvers(t *testing.T) {
	table := []struct {
		rs    []string
		valid bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{""}, false},
		{[]string{"", ""}, false},
		{[]string{"a"}, false},
		{[]string{"a", "b"}, false},
		{[]string{"1.2.3.4"}, true},
		{[]string{"1.2.3.4.5"}, false},
		{[]string{"1.2.3.4", "1.2.3.4"}, false},
		{[]string{"1.2.3.4", "5.6.7.8"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b"}, true},
		{[]string{"2001:db8:3c4d:15::1a2f:1a2b"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b", "2001:db8:3c4d:15::1a2f:1a2b"}, false},
	}
	for i, tc := range table {
		err := validateResolvers(tc.rs)
		if (err == nil && tc.valid) || (err != nil && !tc.valid) {
			continue
		} else if tc.valid {
			t.Fatalf("test %d failed, unexpected error validating resolvers %v: %v", i+1, tc.rs, err)
		} else {
			t.Fatalf("test %d failed, expected validation error for resolvers(%d) %v", i+1, len(tc.rs), tc.rs)
		}
	}
}
