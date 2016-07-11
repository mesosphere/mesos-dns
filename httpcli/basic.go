package httpcli

import (
	"github.com/mesosphere/mesos-dns/httpcli/basic"
	"net/http"
)

// NewBasic wraps an HTTP transactor given basic credentials
func NewBasic(client *http.Client, credentials basic.Credentials) Doer {
	return &basicAuthClient{
		client:      client,
		credentials: credentials,
	}
}

type basicAuthClient struct {
	client      *http.Client
	credentials basic.Credentials
}

// Do implements Doer for iamAuthClient
func (a *basicAuthClient) Do(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(a.credentials.Principal, a.credentials.Secret)

	return a.client.Do(req)
}
