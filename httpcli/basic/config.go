package basic

import (
	"errors"

	"github.com/mesosphere/mesos-dns/httpcli"
)

// ErrInvalidConfiguration generated when Credentials has missing or invalid data
var ErrInvalidConfiguration = errors.New("invalid HTTP Basic configuration")

// Credentials holds a mesos-master principal / secret combination
type Credentials struct {
	Principal string
	Secret    string
}

// Configuration returns a functional option for an httpcli.ConfigMap
func Configuration(c Credentials) httpcli.ConfigMapOption {
	return func(cm httpcli.ConfigMap) {
		cm[httpcli.AuthBasic] = c
	}
}
