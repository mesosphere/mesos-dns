package httpcli

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/mesosphere/mesos-dns/urls"
	"net/http"
	"time"
)

// ErrAuthFailed is returned for any type of IAM authentication failure
var ErrAuthFailed = errors.New("IAM authentication failed")

// Doer executes an http.Request and returns the generated http.Response; similar to http.RoundTripper
// but may modify an in-flight http.Request object.
type Doer interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// Option is a functional option type
type Option func(*http.Client)

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
		if caPool != nil {
			config = &tls.Config{
				RootCAs: caPool,
			}
		} else {
			// do HTTPS without verifying the Mesos master certificate
			config = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}
	return
}
