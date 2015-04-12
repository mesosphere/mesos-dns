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

	// launch DNS server
	if config.DnsOn {
		go func() {
			if err := resolver.LaunchDNS(); err != nil {
				logging.Error.Fatalf("DNS server failed: %v", err)
			} else {
				logging.Error.Fatalf("terminating because DNS died")
			}
		}()
	}

	// launch HTTP server
	if config.HttpOn {
		go func() {
			if err := resolver.LaunchHTTP(); err != nil {
				logging.Error.Fatalf("HTTP server failed: %v", err)
			} else {
				logging.Error.Fatalf("terminating because HTTP died")
			}
		}()
	}

	// launch Zookeeper listener
	if config.Zk != "" {
		if err := resolver.LaunchZK(); err != nil {
			logging.Error.Fatalf("failed to launch ZK listener: %v", err)
		}
	}

	// periodic loading of DNS state (pull from Master)
	resolver.Reload()
	ticker := time.NewTicker(time.Second * time.Duration(config.RefreshSeconds))
	go func() {
		//TODO(jdef) handle panics here?
		for _ = range ticker.C {
			resolver.Reload()
			logging.PrintCurLog()
		}
	}()

	// Wait forever
	select {}
}
