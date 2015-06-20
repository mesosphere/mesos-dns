package labels

import (
	"testing"
)

type boolOrStringType int

const (
	boolType boolOrStringType = iota
	stringType
)

type boolOrString struct {
	vtype   boolOrStringType
	vbool   bool
	vstring string
}

func TestAsDomainFrag(t *testing.T) {
	var (
		trueValue = boolOrString{vbool: true}
		//falseValue = boolOrString{}
	)

	cases := []struct {
		input      string
		output952  string
		output1123 boolOrString
	}{
		{"", "", trueValue},
		{".", "", trueValue},
		{"..", "", trueValue},
		{"...", "", trueValue},
		{"a", "a", trueValue},
		{"abc", "abc", trueValue},
		{"abc.", "abc", trueValue},
		{".abc", "abc", trueValue},
		{"a.c", "a.c", trueValue},
		{".a.c", "a.c", trueValue},
		{"a.c.", "a.c", trueValue},
		{"a..c", "a.c", trueValue},
		{"ab.c", "ab.c", trueValue},
		{"ab.cd", "ab.cd", trueValue},
		{"ab.cd.efg", "ab.cd.efg", trueValue},
		{"a.c.e", "a.c.e", trueValue},
		{"a..c.e", "a.c.e", trueValue},
		{"a.c..e", "a.c.e", trueValue},
		{"pod_123$abc.marathon-0.6.0-dev.mesos", "pod-123abc.marathon-0.dev.mesos", boolOrString{stringType, false, "pod-123abc.marathon-0.6.0-dev.mesos"}},
		{"host.com", "host.com", trueValue},
		{"space space.com", "spacespace.com", trueValue},
		{"blah-dash.com", "blah-dash.com", trueValue},
		{"not$1234.com", "not1234.com", trueValue},
		{"(@ host . com", "host.com", trueValue},
		{"MiXeDcase.CoM", "mixedcase.com", trueValue},
	}
	hostspec952 := ForRFC952()
	hostspec1123 := ForRFC1123()
	for i, tc := range cases {
		act952 := AsDomainFrag(tc.input, hostspec952)
		if act952 != tc.output952 {
			t.Fatalf("expected %q instead of %q for case %d", tc.output952, act952, i)
		}
		act1123 := AsDomainFrag(tc.input, hostspec1123)
		switch tc.output1123.vtype {
		case boolType:
			if !tc.output1123.vbool {
				t.Fatalf("when dns952 output != dns1123 output an expected string should be provided, case %d", i)
			}
			if act1123 != act952 {
				t.Fatalf("expected %q instead of %q for case %d", act952, act1123, i)
			}
		case stringType:
			if act1123 != tc.output1123.vstring {
				t.Fatalf("expected %q instead of %q for case %d", tc.output1123.vstring, act1123, i)
			}
		}
	}
}
