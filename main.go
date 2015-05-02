package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/util"
)

var (
	cjson       = flag.String("config", "config.json", "path to config file (json)")
	versionFlag = flag.Bool("version", false, "output the version")
)

func main() {
	util.PanicHandlers = append(util.PanicHandlers, func(_ interface{}) {
		// by default the handler already logs the panic
		os.Exit(1)
	})

	// parse flags
	flag.Parse()

	// -version
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// initialize logging
	logging.SetupLogs()

	// print error and terminate
	eh := func(name string, err error) {
		if err != nil {
			logging.Error.Fatalf("%s failed: %v", name, err)
		} else {
			logging.Error.Fatalf("%s stopped unexpectedly", name)
		}
	}

	// run mesos-dns
	app := newApp(eh)
	app.run()
}
