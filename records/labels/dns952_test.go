package labels

import (
	"testing"
)

func BenchmarkAsDNS952(b *testing.B) {
	const (
		original = "89f.gsf---g_7-fgs--d7fddg-"
		expected = "f-gsf---g-7-fgs--d7fddg"
	)

	// run the AsDNS952 func b.N times
	for n := 0; n < b.N; n++ {
		if actual := AsDNS952(original); actual != expected {
			b.Fatalf("expected %q instead of %q", expected, actual)
		}
	}
}

func TestAsDNS952(t *testing.T) {
	tests := map[string]string{
		"":                                "",
		"a":                               "a",
		"-":                               "",
		"a---":                            "a",
		"---a---":                         "a",
		"---a---b":                        "a---b",
		"a.b.c.d.e":                       "a-b-c-d-e",
		"a.c.d_de.":                       "a-c-d-de",
		"abc123":                          "abc123",
		"4abc123":                         "abc123",
		"-abc123":                         "abc123",
		"abc123-":                         "abc123",
		"abc-123":                         "abc-123",
		"abc--123":                        "abc--123",
		"fd%gsf---gs7-f$gs--d7fddg-123":   "fdgsf---gs7-fgs--d7fddg1",
		"89fdgsf---gs7-fgs--d7fddg-123":   "fdgsf---gs7-fgs--d7fddg1",
		"89fdgsf---gs7-fgs--d7fddg---123": "fdgsf---gs7-fgs--d7fddg1",
		"89fdgsf---gs7-fgs--d7fddg-":      "fdgsf---gs7-fgs--d7fddg",
	}
	for untrusted, expected := range tests {
		actual := AsDNS952(untrusted)
		if actual != expected {
			t.Fatalf("expected %q instead of %q after converting %q", expected, actual, untrusted)
		}
	}
}
