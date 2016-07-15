package main

import (
	"github.com/mesosphere/mesos-dns/httpcli/basic"
	"github.com/mesosphere/mesos-dns/httpcli/iam"
)

// initAuth registers HTTP client factories for supported authentication types
func initAuth() {
	basic.Register()
	iam.Register()
}
