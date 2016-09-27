package records

import (
	"testing"
)

func TestValidateDomain(t *testing.T) {
	testDomain := func(domain string, shouldSucceed bool) {
		err := validateDomainName(domain)
		if shouldSucceed && err != nil {
			t.Errorf("validation should have succeeded for %s", domain)
		}
		if !shouldSucceed && err == nil {
			t.Errorf("validation should have failed for %s", domain)
		}
	}

	testDomain("", false)
	testDomain("1-awesome.domain.e", true)
	testDomain(".invalid.com", false)
	testDomain("invalid.com.", false)
	testDomain("single-name", true)
	testDomain("name", true)
}

func TestValidateMasters(t *testing.T) {
	for i, tc := range []validationTest{
		{nil, true},
		{[]string{}, true},
		{[]string{""}, false},
		{[]string{"", ""}, false},
		{[]string{"a"}, false},
		{[]string{"a:1234"}, true},
		{[]string{"a", "b"}, false},
		{[]string{"a:1", "b:1"}, true},
		{[]string{"a:0"}, false},
		{[]string{"a:65535"}, true},
		{[]string{"a:65536"}, false},
		{[]string{"1.2.3.4"}, false},
		{[]string{"1.2.3.4:5"}, true},
		{[]string{"1.2.3.4.5"}, false},
		{[]string{"1.2.3.4.5:6"}, true}, // no validation of hostnames
		{[]string{"1.2.3.4", "1.2.3.4"}, false},
		{[]string{"1.2.3.4:1", "1.2.3.4:1"}, false},
		{[]string{"1.2.3.4:1", "5.6.7.8:1"}, true},
		{[]string{"[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:1"}, true},
		{[]string{"[2001:db8:3c4d:15::1a2f:1a2b]:1"}, true},
		{[]string{"[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:1", "[2001:db8:3c4d:15::1a2f:1a2b]:1"}, false},
	} {
		validate(t, i+1, tc, validateMasters)
	}
}

func TestValidateResolvers(t *testing.T) {
	for i, tc := range []validationTest{
		{nil, true},
		{[]string{}, true},
		{[]string{""}, false},
		{[]string{"", ""}, false},
		{[]string{"a"}, false},
		{[]string{"a", "b"}, false},
		{[]string{"1.2.3.4"}, true},
		{[]string{"1.2.3.4:53"}, true},
		{[]string{"1.2.3.4.5"}, false},
		{[]string{"1.2.3.4", "1.2.3.4"}, false},
		{[]string{"1.2.3.4", "5.6.7.8"}, true},
		{[]string{"1.2.3.4", "5.6.7.8:1234"}, true},
		{[]string{"5.6.7.8:-1"}, false},
		{[]string{"5.6.7.8:65535"}, true},
		{[]string{"5.6.7.8:65536"}, false},
		{[]string{"5.6.7.8:abc"}, false},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b"}, true},
		{[]string{"2001:db8:3c4d:15::1a2f:1a2b"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b", "[2001:db8:3c4d:15::1a2f:1a2b]:55"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b", "2001:db8:3c4d:15::1a2f:1a2b"}, false},
	} {
		validate(t, i+1, tc, validateResolvers)
	}
}

type validationTest struct {
	in    []string
	valid bool
}

func validate(t *testing.T, i int, tc validationTest, f func([]string) error) {
	switch err := f(tc.in); {
	case (err == nil && tc.valid) || (err != nil && !tc.valid):
		return // valid
	case tc.valid:
		t.Fatalf("test %d failed, unexpected error validating resolvers %v: %v", i, tc.in, err)
	default:
		t.Fatalf("test %d failed, expected validation error for resolvers(%d) %v", i, len(tc.in), tc.in)
	}
}

func TestValidateZoneResolvers(t *testing.T) {
	ips := []string{"8.8.8.8"}

	fn := func(zrs map[string][]string) error {
		return validateZoneResolvers(zrs, "dc.mesos")
	}

	for i, tc := range []zoneValidationTest{
		{nil, true},
		{map[string][]string{"": ips}, false},
		{map[string][]string{"weave": ips}, true},
		{map[string][]string{"weave": []string{}}, false},
		{map[string][]string{"mesos": ips}, false},
		{map[string][]string{"dc.mesos": ips}, false},
		{map[string][]string{"acdc.mesos": ips}, true},
		{map[string][]string{"site.dc.mesos": ips}, false},
		{map[string][]string{"abc.com": ips, "com": ips}, false},
		{map[string][]string{"abc.com": ips, "bc.com": ips}, true},
	} {
		validateZone(t, i+1, tc, fn)
	}
}

type zoneValidationTest struct {
	in    map[string][]string
	valid bool
}

func validateZone(t *testing.T, i int, tc zoneValidationTest,
	f func(map[string][]string) error) {
	switch err := f(tc.in); {
	case (err == nil && tc.valid) || (err != nil && !tc.valid):
		return // valid
	case tc.valid:
		t.Fatalf("test %d failed, unexpected error validating zone resolvers "+
			"%v: %v", i, tc.in, err)
	default:
		t.Fatalf("test %d failed, expected validation error for zone resolvers(%d)"+
			" %v", i, len(tc.in), tc.in)
	}
}
