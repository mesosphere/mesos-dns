package labels

import (
	"testing"
)

func BenchmarkAsDNS952(b *testing.B) {
	const (
		original = "89f.gsf---g_7-fgs--d7fddg-"
		expected = "f-gsf---g-7-fgs--d7fddg"
	)

	hostspec952 := ForRFC952()
	// run the mangle func b.N times
	for n := 0; n < b.N; n++ {
		if actual := hostspec952.Mangle(original); actual != expected {
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
		"r29f.dev.angrypigs":              "r29f-dev-angrypigs",
	}
	hostspec952 := ForRFC952()
	for untrusted, expected := range tests {
		actual := hostspec952.Mangle(untrusted)
		if actual != expected {
			t.Fatalf("expected %q instead of %q after converting %q", expected, actual, untrusted)
		}
	}
}

func BenchmarkAsDNS1123(b *testing.B) {
	const (
		original = "##fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789-"
		expected = "fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789"
	)

	hostspec1123 := ForRFC1123()
	// run the asDNS1123 func b.N times
	for n := 0; n < b.N; n++ {
		if actual := hostspec1123.Mangle(original); actual != expected {
			b.Fatalf("expected %q instead of %q", expected, actual)
		}
	}
}

func TestAsDNS1123(t *testing.T) {
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
		"4abc123":                         "4abc123",
		"-abc123":                         "abc123",
		"abc123-":                         "abc123",
		"abc-123":                         "abc-123",
		"abc--123":                        "abc--123",
		"fd%gsf---gs7-f$gs--d7fddg-123":   "fdgsf---gs7-fgs--d7fddg-123",
		"89fdgsf---gs7-fgs--d7fddg-123":   "89fdgsf---gs7-fgs--d7fddg-123",
		"89fdgsf---gs7-fgs--d7fddg---123": "89fdgsf---gs7-fgs--d7fddg---123",
		"89fdgsf---gs7-fgs--d7fddg-":      "89fdgsf---gs7-fgs--d7fddg",

		"fd%gsf---gs7-f$gs--d7fddg123456789012345678901234567890123456789-123":   "fdgsf---gs7-fgs--d7fddg1234567890123456789012345678901234567891",
		"$$fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789-123":   "fdgsf---gs7-fgs--d7fddg1234567890123456789012345678901234567891",
		"%%fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789---123": "fdgsf---gs7-fgs--d7fddg1234567890123456789012345678901234567891",
		"##fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789-":      "fdgsf---gs7-fgs--d7fddg123456789012345678901234567890123456789",

		"r29f.dev.angrypigs": "r29f-dev-angrypigs",
	}
	hostspec1123 := ForRFC1123()
	for untrusted, expected := range tests {
		actual := hostspec1123.Mangle(untrusted)
		if actual != expected {
			t.Fatalf("expected %q instead of %q after converting %q", expected, actual, untrusted)
		}
	}
}
