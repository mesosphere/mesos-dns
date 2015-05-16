package records

import (
	"fmt"
	"net"
)

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
