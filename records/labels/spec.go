package labels

const (
	// separatorChar delimits concatenated labels.
	separatorChar = '.'

	dns952MaxLength  int = 24
	dns1123MaxLength int = 63
)

var (
	hostNameSpec952 = hostNameSpec{
		table:  makeDNS952Table(),
		maxlen: dns952MaxLength,
	}
	hostNameSpec1123 = hostNameSpec{
		table:  makeDNS1123Table(),
		maxlen: dns1123MaxLength,
	}
)

// ForRFC952 returns a HostNameSpec that satisfies the DNS label rules
// specified in RFC952. See http://www.rfc-base.org/txt/rfc-952.txt
func ForRFC952() HostNameSpec {
	return &hostNameSpec952
}

// ForRFC1123 returns a HostNameSpec that satisfies the DNS label rules
// specified in RFC1123. See http://www.rfc-base.org/txt/rfc-1123.txt
func ForRFC1123() HostNameSpec {
	return &hostNameSpec1123
}

// HostNameSpec implements rules related to a particular host name specification standard
type HostNameSpec interface {
	// Mangle transforms an input string to produce an output string that is
	// compliant with the specification. If the input string is already
	// compliant then it is returned unchanged.
	Mangle(string) string
}

type hostNameSpec struct {
	table  dnsCharTable
	maxlen int
}

func (spec *hostNameSpec) Mangle(in string) string {
	return spec.table.toLabel(in, spec.maxlen)
}
