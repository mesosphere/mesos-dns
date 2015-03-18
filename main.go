package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/resolver"

	"github.com/miekg/dns"
)

func main() {
	var wg sync.WaitGroup
	var resolver resolver.Resolver

	versionFlag := false

	cjson := flag.String("config", "config.json", "location of configuration file (json)")
	flag.BoolVar(&logging.VerboseFlag, "e", false, "verbose logging")
	flag.BoolVar(&logging.VeryVerboseFlag, "ee", false, "very verbose logging")
	flag.BoolVar(&versionFlag, "version", false, "output the version")
	flag.Parse()

	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	logging.SetupLogs()

	resolver.Config = records.SetConfig(*cjson)

	// if ZK is identified, start detector
	if resolver.Config.Zk[0] != "" {
		records.ZKdetect(resolver.Config)
	}

	// reload the first time
	resolver.Reload()
	ticker := time.NewTicker(time.Second * time.Duration(resolver.Config.RefreshSeconds))
	go func() {
		for _ = range ticker.C {
			resolver.Reload()
			logging.PrintCurLog()
		}
	}()

	// handle for everything in this domain...
	dns.HandleFunc(resolver.Config.Domain+".", panicRecover(resolver.HandleMesos))
	dns.HandleFunc(".", panicRecover(resolver.HandleNonMesos))

	go resolver.Serve("tcp")
	go resolver.Serve("udp")

	wg.Add(1)
	wg.Wait()
}

// panicRecover catches any panics from the resolvers and sets an error
// code of server failure
func panicRecover(f func(w dns.ResponseWriter, r *dns.Msg)) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer func() {
			if rec := recover(); rec != nil {
				m := new(dns.Msg)
				m.SetReply(r)
				m.SetRcode(r, 2)
				_ = w.WriteMsg(m)
				logging.Error.Println(rec)
			}
		}()
		f(w, r)
	}
}
