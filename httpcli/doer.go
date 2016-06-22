package httpcli

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mesosphere/mesos-dns/errorutil"
	"github.com/mesosphere/mesos-dns/httpcli/iam"
	"github.com/mesosphere/mesos-dns/urls"
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

// New generates and returns an HTTP transactor given an optional IAM configuration and some set of
// functional options.
func New(maybeConfig *iam.Config, options ...Option) Doer {
	defaultClient := &http.Client{}
	for i := range options {
		if options[i] != nil {
			options[i](defaultClient)
		}
	}
	if maybeConfig != nil {
		authClient := &authClient{
			client: defaultClient,
			config: maybeConfig,
		}
		return authClient
	}
	return defaultClient
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

type authClient struct {
	client *http.Client
	config *iam.Config
}

// Do implements Doer for authClient
func (a *authClient) Do(req *http.Request) (*http.Response, error) {
	// TODO if we still have a valid token, try using it first
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims["uid"] = a.config.ID
	token.Claims["exp"] = time.Now().Add(time.Hour).Unix()
	// SignedString will treat secret as PEM-encoded key
	tokenStr, err := token.SignedString([]byte(a.config.Secret))
	if err != nil {
		return nil, err
	}

	authReq := struct {
		UID   string `json:"uid"`
		Token string `json:"token,omitempty"`
	}{
		UID:   a.config.ID,
		Token: tokenStr,
	}

	b, err := json.Marshal(authReq)
	if err != nil {
		return nil, err
	}

	authBody := bytes.NewBuffer(b)
	resp, err := a.client.Post(a.config.LoginEndpoint, "application/json", authBody)
	if err != nil {
		return nil, err
	}
	defer errorutil.Ignore(resp.Body.Close)
	if resp.StatusCode != 200 {
		return nil, ErrAuthFailed
	}

	var authResp struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&authResp)
	if err != nil {
		return nil, err
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("Authorization", "token="+authResp.Token)

	return a.client.Do(req)
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
