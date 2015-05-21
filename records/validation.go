package records

import (
	"fmt"
	"net"
)

// validateMasters checks that each master in the list is a properly formatted host:ip pair.
// duplicate masters in the list are not allowed.
// returns nil if the masters list if empty, or else all masters in the list are valid.
func validateMasters(ms []string) error {
	valid := map[string]struct{}{}
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

// validateResolvers checks that each resolver in the list is a properly formatted IP address.
// duplicate resolvers in the list are not allowed.
// returns nil if the resolver list is empty, or else all resolvers in the list are valid.
func validateResolvers(rs []string) error {
	ips := map[string]struct{}{}
	for _, r := range rs {
		ip := net.ParseIP(r)
		if ip == nil {
			return fmt.Errorf("illegal IP specified for resolver %q", r)
		}
		ipstr := ip.String()
		if _, found := ips[ipstr]; found {
			return fmt.Errorf("duplicate resolver IP specified: %v", r)
		}
		ips[ipstr] = struct{}{}
	}
	return nil
}
