package records

import (
	"fmt"
	"net"
	"strconv"
)

func validateEnabledServices(c *Config) error {
	if !c.DNSOn && !c.HTTPOn {
		return fmt.Errorf("Either DNS or HTTP server should be on")
	}
	if len(c.Masters) == 0 && c.Zk == "" {
		return fmt.Errorf("Specify Mesos masters or Zookeeper in config.json")
	}
	return nil
}

// validateMasters checks that each master in the list is a properly formatted host:port or IP:port pair.
// duplicate masters in the list are not allowed.
// returns nil if the masters list is empty, or else all masters in the list are valid.
func validateMasters(ms []string) error {
	if err := validateUniqueStrings(ms, normalizeMaster); err != nil {
		return fmt.Errorf("Error validating masters: %v", err)
	}
	return nil
}

func normalizeMaster(hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", fmt.Errorf("Illegal host:port specified: %v. Error: %v", hostPort, err)
	}
	if ip := net.ParseIP(host); ip != nil {
		//TODO(jdef) distinguish between intended hostnames and invalid ip addresses
		host = ip.String()
	}
	if !validPortString(port) {
		return "", fmt.Errorf("Illegal host:port specified: %v", hostPort)
	}
	return net.JoinHostPort(host, port), nil
}

// validateResolvers checks that each resolver in the list is a properly formatted IP or IP:port pair.
// duplicate resolvers in the list are not allowed.
// returns nil if the resolver list is empty, or else all resolvers in the list are valid.
func validateResolvers(rs []string) error {
	if err := validateUniqueStrings(rs, normalizeResolver); err != nil {
		return fmt.Errorf("Error validating resolvers: %v", err)
	}
	return nil
}

func normalizeResolver(hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
		port = "53"
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		return "", fmt.Errorf("Illegal ip specified: %v", host)
	}

	if !validPortString(port) {
		return "", fmt.Errorf("Illegal host:port specified: %v", hostPort)
	}
	return net.JoinHostPort(host, port), nil
}

// validateUniqueStrings runs a normalize function on each string in a list and
// retuns an error if any duplicates are found.
func validateUniqueStrings(strings []string, normalize func(string) (string, error)) error {
	valid := make(map[string]struct{}, len(strings))
	for _, str := range strings {
		normalized, err := normalize(str)
		if err != nil {
			return err
		}
		if _, found := valid[normalized]; found {
			return fmt.Errorf("Duplicate found: %v", str)
		}
		valid[normalized] = struct{}{}
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
		case "host", "docker", "mesos", "netinfo":
		default:
			return fmt.Errorf("invalid ip source %q", src)
		}
	}

	return nil
}

// validPortString retuns true if the given port string is
// an integer between 1 and 65535, false otherwise.
func validPortString(portString string) bool {
	port, err := strconv.Atoi(portString)
	return err == nil && port > 0 && port <= 65535
}
