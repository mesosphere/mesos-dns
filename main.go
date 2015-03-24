package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
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
	flag.BoolVar(&versionFlag, "version", false, "output the version")
	flag.Parse()

	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if glog.V(2) {
		logging.VeryVerboseFlag = true
	} else if glog.V(1) {
		logging.VerboseFlag = true
	}

	logging.SetupLogs()

	resolver.Version = version
	resolver.Config = records.SetConfig(*cjson)

	// handle for everything in this domain...
	dns.HandleFunc(resolver.Config.Domain+".", panicRecover(resolver.HandleMesos))
	dns.HandleFunc(".", panicRecover(resolver.HandleNonMesos))

	go resolver.Serve("tcp")
	go resolver.Serve("udp")
	go resolver.Hdns()


	// if ZK is identified, start detector and wait for first master
	if resolver.Config.Zk != "" {
		dr, err := records.ZKdetect(&resolver.Config)
		if err != nil {
			logging.Error.Println(err.Error())
			os.Exit(1)
		}

		logging.VeryVerbose.Println("Warning: waiting for initial information from Zookeper.")
		select {
		case <-dr:
			logging.VeryVerbose.Println("Warning: done waiting for initial information from Zookeper.")
		case <-time.After(2 * time.Minute):
			logging.Error.Println("timed out waiting for initial ZK detection, exiting")
			os.Exit(1)
		}
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
