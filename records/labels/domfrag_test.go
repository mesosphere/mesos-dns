package labels

import (
	"testing"
)

func TestAsDomainFrag(t *testing.T) {
	cases := map[string]string{
		"":          "",
		".":         "",
		"..":        "",
		"...":       "",
		"a":         "a",
		"abc":       "abc",
		"abc.":      "abc",
		".abc":      "abc",
		"a.c":       "a.c",
		".a.c":      "a.c",
		"a.c.":      "a.c",
		"a..c":      "a.c",
		"ab.c":      "ab.c",
		"ab.cd":     "ab.cd",
		"ab.cd.efg": "ab.cd.efg",
		"a.c.e":     "a.c.e",
		"a..c.e":    "a.c.e",
		"a.c..e":    "a.c.e",
		"pod_123$abc.marathon-0.6.0-dev.mesos": "pod-123abc.marathon-0.dev.mesos",
		"host.com":                             "host.com",
		"space space.com":                      "spacespace.com",
		"blah-dash.com":                        "blah-dash.com",
		"not$1234.com":                         "not1234.com",
		"(@ host . com":                        "host.com",
		"MiXeDcase.CoM":                        "mixedcase.com",
	}
	for orig, exp := range cases {
		act := AsDomainFrag(orig)
		if act != exp {
			t.Fatalf("expected %q instead of %q for case %q", exp, act, orig)
		}
	}
}
