package iam

// Config captures the configuration that allows mesos-dns to authenticate against some
// IAM endpoint.
type Config struct {
	ID            string `json:"uid"`            // ID
	Secret        string `json:"secret"`         // Secret
	Password      string `json:"password"`       // Password
	LoginEndpoint string `json:"login_endpoint"` // LoginEndpoint
}
