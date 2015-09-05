package records

import (
	"fmt"
	"net"
)

// validateMasters checks that each master in the list is a properly formatted host:ip pair.
// duplicate masters in the list are not allowed.
// returns nil if the masters list is empty, or else all masters in the list are valid.
func validateMasters(ms []string) error {
	if len(ms) == 0 {
		return nil
	}
	valid := make(map[string]struct{}, len(ms))
	for i, m := range ms {
		h, p, err := net.SplitHostPort(m)
		if err != nil {
			return fmt.Errorf("illegal host:port specified for master %q", ms[i])
		}
		// normalize ipv6 addresses
		if ip := net.ParseIP(h); ip != nil {
			h = ip.String()
			m = h + "_" + p
		}
		//TODO(jdef) distinguish between intended hostnames and invalid ip addresses
		if _, found := valid[m]; found {
			return fmt.Errorf("duplicate master specified: %v", ms[i])
		}
		valid[m] = struct{}{}
	}
	return nil
}

// validateResolvers errors if there are duplicate resolvers, otherwise returns nil.
func validateResolvers(rs []string) error {
	if len(rs) == 0 {
		return nil
	}
	ips := make(map[string]struct{}, len(rs))
	for _, r := range rs {
		if _, found := ips[r]; found {
			return fmt.Errorf("duplicate resolver IP specified: %v", r)
		}
		ips[r] = struct{}{}
	}
	return nil
}

// validateIPSources checks validity of ip sources
func validateIPSources(srcs []string) error {
	if len(srcs) == 0 {
		return fmt.Errorf("empty ip sources")
	}
	if len(srcs) != len(unique(srcs)) {
		return fmt.Errorf("duplicate ip source specified")
	}
	for _, src := range srcs {
		switch src {
		case "host", "docker", "mesos":
		default:
			return fmt.Errorf("invalid ip source %q", src)
		}
	}

	return nil
}
