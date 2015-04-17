package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/resolver"
	"github.com/mesosphere/mesos-dns/util"
)

const (
	zkInitialDetectionTimeout = 4 * time.Minute
)

func main() {
	util.PanicHandlers = append(util.PanicHandlers, func(_ interface{}) {
		// by default the handler already logs the panic
		os.Exit(1)
	})

	var versionFlag bool

	// parse flags
	cjson := flag.String("config", "config.json", "path to config file (json)")
	flag.BoolVar(&versionFlag, "version", false, "output the version")
	flag.Parse()

	// -version
	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// initialize logging
	logging.SetupLogs()

	// initialize resolver
	config := records.SetConfig(*cjson)
	resolver := resolver.New(version, config)

	var dnsErr, httpErr, zkErr <-chan error
	var newLeader <-chan struct{}

	// launch DNS server
	if config.DnsOn {
		dnsErr = resolver.LaunchDNS()
	}

	// launch HTTP server
	if config.HttpOn {
		httpErr = resolver.LaunchHTTP()
	}

	// launch Zookeeper listener
	if config.Zk != "" {
		newLeader, zkErr = resolver.LaunchZK(zkInitialDetectionTimeout)
	}

	handleServerErr := func(name string, err error) {
		if err != nil {
			logging.Error.Fatalf("%s failed: %v", name, err)
		} else {
			logging.Error.Fatalf("%s stopped unexpectedly", name)
		}
	}

	reloadSignal := make(chan struct{}, 1)
	tryReload := func() {
		// non-blocking, attempt to queue a reload
		select {
		case reloadSignal <- struct{}{}:
		default:
		}
	}

	// periodic loading of DNS state (pull from Master)
	go func() {
		defer util.HandleCrash()
		reloadTimeout := time.Second * time.Duration(config.RefreshSeconds)
		reloadTimer := time.AfterFunc(reloadTimeout, tryReload)
		for range reloadSignal {
			resolver.Reload()
			logging.PrintCurLog()
			reloadTimer.Reset(reloadTimeout)
		}
	}()

	tryReload()

	for {
		select {
		case <-newLeader:
			tryReload()
		case err := <-dnsErr:
			handleServerErr("DNS server", err)
		case err := <-httpErr:
			handleServerErr("HTTP server", err)
		case err := <-zkErr:
			handleServerErr("ZK watcher", err)
		}
	}
}
