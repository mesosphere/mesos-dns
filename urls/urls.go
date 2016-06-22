package urls

import (
	"net/url"
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
