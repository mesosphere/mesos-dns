package urls

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Option is a functional type that configures a URL
type Option func(*Builder)

// Builder represents a URL under construction
type Builder url.URL

// With configures a net.URL via a Builder wrapper, returns the modified result
func (b Builder) With(options ...Option) Builder {
	for i := range options {
		if options[i] != nil {
			options[i](&b)
		}
	}
	return b
}

// Scheme returns an Option to configure a Builder.Scheme
func Scheme(scheme string) Option { return func(b *Builder) { b.Scheme = scheme } }

// Host returns an Option to configure a Builder.Host
func Host(host string) Option { return func(b *Builder) { b.Host = host } }

// Path returns an Option to configure a Builder.Path
func Path(path string) Option { return func(b *Builder) { b.Path = path } }

// SplitHostPort should be able to accept
//     ip:port
//     zk://host1:port1,host2:port2,.../path
//     zk://username:password@host1:port1,host2:port2,.../path
//     file:///path/to/file (where file contains one of the above)
func SplitHostPort(pair string) (string, string, error) {
	if host, port, err := net.SplitHostPort(pair); err == nil {
		return host, port, nil
	}

	h := strings.SplitN(pair, ":", 2)
	if len(h) != 2 {
		return "", "", fmt.Errorf("unable to parse proto from %q", pair)
	}
	return h[0], h[1], nil
}
