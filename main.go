package main

import (
	"flag"
	"fmt"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/resolver"
	"os"
	"time"
)

func main() {
	var resolver resolver.Resolver
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
	resolver.Config = records.SetConfig(*cjson)
	resolver.Version = version

	// launch DNS server
	if resolver.Config.DnsOn {
		resolver.LaunchDNS()
	}

	// launch HTTP server
	if resolver.Config.HttpOn {
		go resolver.LaunchHTTP()
	}

	// launch Zookeeper listener
	if resolver.Config.Zk != "" {
		resolver.LaunchZK()
	}

	// periodic loading of DNS state (pull from Master)
	resolver.Reload()
	ticker := time.NewTicker(time.Second * time.Duration(resolver.Config.RefreshSeconds))
	go func() {
		for _ = range ticker.C {
			resolver.Reload()
			logging.PrintCurLog()
		}
	}()

	// Wait forever
	select {}
}
