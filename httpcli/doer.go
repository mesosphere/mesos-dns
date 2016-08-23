package httpcli

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mesosphere/mesos-dns/urls"
)

// ErrAuthFailed is returned for any type of IAM authentication failure
var ErrAuthFailed = errors.New("IAM authentication failed")

// Doer executes an http.Request and returns the generated http.Response; similar to http.RoundTripper
// but may modify an in-flight http.Request object.
type Doer interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// DoerFunc is the functional adaptation of Doer
type DoerFunc func(req *http.Request) (resp *http.Response, err error)

// Do implements Doer for DoerFunc
func (df DoerFunc) Do(req *http.Request) (*http.Response, error) { return df(req) }

// DoerFactory generates a Doer. If the given Client is nil then the returned Doer must also be nil.
// Specifying a nil Client is useful for asking the factory to ONLY validate the provided ConfigMap.
type DoerFactory func(ConfigMap, *http.Client) Doer

// Option is a functional option type
type Option func(*http.Client)

// New generates and returns an HTTP transactor given an optional IAM configuration and some set of
// functional options.
func New(am AuthMechanism, cm ConfigMap, options ...Option) Doer {
	defaultClient := &http.Client{}
	for i := range options {
		if options[i] != nil {
			options[i](defaultClient)
		}
	}

	df, ok := factoryFor(am)
	if !ok {
		panic(fmt.Sprintf("unregistered auth mechanism %q", am))
	}
	return df(cm, defaultClient)
}

// Timeout returns an Option that configures client timeout
func Timeout(timeout time.Duration) Option {
	return func(client *http.Client) {
		client.Timeout = timeout
	}
}

// Transport returns an Option that configures client transport
func Transport(tr http.RoundTripper) Option {
	return func(client *http.Client) {
		client.Transport = tr
	}
}

// TLSConfig generates and returns a recommended URL generation option and TLS configuration.
func TLSConfig(enabled bool, caPool *x509.CertPool) (opt urls.Option, config *tls.Config) {
	opt = urls.Scheme("http")
	if enabled {
		opt = urls.Scheme("https")
		config = &tls.Config{
			RootCAs:            caPool,
			InsecureSkipVerify: caPool == nil,
		}
	}
	return
}
