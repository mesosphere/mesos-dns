package main

import (
	"time"

	"github.com/mesosphere/mesos-dns/plugins"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/records/config"
	"github.com/mesosphere/mesos-dns/resolver"
)

const (
	zkInitialDetectionTimeout = 30 * time.Second
)

type errorHandlerFunc func(string, error)

type app struct {
	config       records.Config
	resolver     *resolver.Resolver
	filters      plugins.FilterSet
	ready        chan struct{} // when closed, indicates that initialization has completed
	done         chan struct{} // when closed, indicates that run has completed
	errHandler   errorHandlerFunc
	recordConfig *config.RecordConfig
}

func newApp(eh errorHandlerFunc) *app {
	c := &app{
		errHandler: eh,
		ready:      make(chan struct{}),
		done:       make(chan struct{}),
	}
	c.initialize()
	return c
}

func (c *app) initialize() {
	select {
	case <-c.ready:
		panic("app already initialized")
	default:
		defer close(c.ready)
	}

	c.config = records.SetConfig(*cjson)
	c.resolver = resolver.New(version, c.config)

	c.recordConfig = config.New(config.RecordConfigNotificationSnapshotAndUpdates)
	masterSource := config.NewSourceMaster(c.config, c.errHandler, c.recordConfig.Channel(records.MasterSource))
	pctx := newPluginContext(c, masterSource)

	// launch http plugin first, so that we claim the endpoint namespace that we want
	if c.config.HttpOn {
		pctx.launchPlugin("HTTP server", resolver.NewAPIPlugin(c.resolver))
	}

	pctx.initAddonPlugins()

	// this is launched last because other plugins may have registered filters
	if c.config.DnsOn {
		pctx.launchPlugin("DNS server", resolver.NewDNSPlugin(c.resolver, c.filters.Apply))
	}
}

func (c *app) run() {
	select {
	case <-c.ready:
		select {
		case <-c.done:
			panic("run already completed")
		default:
			defer close(c.done)
		}
	default:
		panic("cannot run, not yet initialized")
	}

	//TODO(jdef) we probably want to wait for plugin teardown upon this method returning, and
	//then exit gracefully. ala OnSignal(c.done, c.pluginWaitGroup.Wait()).Then(func() { os.Exit(0) })

	// block until the updates chan is closed
	c.resolver.Run(c.recordConfig.Updates())
}
