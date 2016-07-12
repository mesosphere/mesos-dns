package basic

import (
	"fmt"
	"net/http"

	"github.com/mesosphere/mesos-dns/httpcli"
)

// Register registers a DoerFactory for HTTP Basic authentication
func Register() {
	httpcli.Register(httpcli.AuthBasic, httpcli.DoerFactory(func(cm httpcli.ConfigMap, c *http.Client) httpcli.Doer {
		obj := cm.FindOrPanic(httpcli.AuthBasic)
		config, ok := obj.(Credentials)
		if !ok {
			panic(fmt.Sprintf("expected Credentials instead of %#+v", obj))
		}
		return Doer(c, config)
	}))
}

// Doer wraps an HTTP transactor given basic credentials
func Doer(client httpcli.Doer, credentials Credentials) httpcli.Doer {
	return httpcli.DoerFunc(func(req *http.Request) (*http.Response, error) {
		req.SetBasicAuth(credentials.Principal, credentials.Secret)
		return client.Do(req)
	})
}
