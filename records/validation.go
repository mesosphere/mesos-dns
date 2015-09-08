package records

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
)

const hostnamePattern = `^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`

var hostnameRegexp *regexp.Regexp

// validateMasters checks that each element in the list is valid.
// List elements must be either a valid IP:port or a valid hostname:port.
// List must not contain duplicates (after IPv6 normalization).
func validateMasters(ms []string) error {
	if len(ms) == 0 {
		return nil
	}
	valid := make(map[string]struct{}, len(ms))
	for i, m := range ms {
		h, p, err := net.SplitHostPort(m)
		if err != nil {
			return fmt.Errorf("invalid host:port specified for master %q", ms[i])
		}
		if ip := net.ParseIP(h); ip != nil {
			// normalize ipv6 addresses
			m = net.JoinHostPort(ip.String(), p)
		} else if !isHostname(h) {
			return fmt.Errorf("invalid hostname or IP specified for master %q", ms[i])
		}
		if _, found := valid[m]; found {
			return fmt.Errorf("duplicate master specified: %q", ms[i])
		}
		valid[m] = struct{}{}
	}
	return nil
}

// validateResolvers checks that each element in the list is valid.
// List elements must be either a valid IP or a valid hostname.
// List must not contain duplicates (after IPv6 normalization).
func validateResolvers(rs []string) error {
	if len(rs) == 0 {
		return nil
	}
	ips := make(map[string]struct{}, len(rs))
	for _, r := range rs {
		if ip := net.ParseIP(r); ip != nil {
			// normalize ipv6 addresses
			r = ip.String()
		} else if !isHostname(r) {
			return fmt.Errorf("invalid hostname or IP specified for resolver %q", r)
		}
		if _, found := ips[r]; found {
			return fmt.Errorf("duplicate resolver specified: %q", r)
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

func isHostname(host string) bool {
	if hostnameRegexp == nil {
		hostnameRegexp = regexp.MustCompile(hostnamePattern)
	}
	matchSubstrings := hostnameRegexp.FindStringSubmatch(host)
	if matchSubstrings == nil {
		return false
	}
	tld := matchSubstrings[len(matchSubstrings)-1]
	// TLD must not be all digits
	if _, err := strconv.Atoi(tld); err == nil {
		return false
	}
	return true
}
