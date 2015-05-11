package main

import (
	"github.com/emicklei/go-restful"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/plugins"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/records/config"
)

type pluginContext struct {
	*app
	onPreload  func(config.RecordLoader)
	onPostload func(config.RecordLoader)
}

func newPluginContext(c *app, master *config.MasterSource) *pluginContext {
	return &pluginContext{
		app:        c,
		onPreload:  master.OnPreload,
		onPostload: master.OnPostload,
	}
}

func (c *pluginContext) Events() plugins.RecordEvents {
	return c
}

func (c *pluginContext) Done() <-chan struct{} {
	// clients that use this chan will block until run completes
	return c.done
}

// implements plugin.RecordEvents interface, panics if invoked outside of initialization process
func (c *pluginContext) OnPreload(r plugins.RecordLoader) {
	select {
	case <-c.ready:
		panic("cannot OnPreload after initialization has completed")
	default:
		c.onPreload(config.RecordLoader(r))
	}
}

func (c *pluginContext) OnPostload(r plugins.RecordLoader) {
	select {
	case <-c.ready:
		panic("cannot OnPostload after initialization has completed")
	default:
		c.onPostload(config.RecordLoader(r))
	}
}

func (c *pluginContext) AddFilter(f plugins.Filter) {
	select {
	case <-c.ready:
		panic("cannot AddFilter after initialization has completed")
	default:
	}
	if f != nil {
		c.filters = append(c.filters, f)
	}
}

func (c *pluginContext) RegisterSource(source string) chan<- interface{} {
	select {
	case <-c.ready:
		panic("cannot RegisterSource after initialization has completed")
	default:
		return c.recordConfig.Channel(source)
	}
}

func (c *pluginContext) RegisterWS(ws *restful.WebService) {
	select {
	case <-c.ready:
		panic("cannot RegisterWS after initialization has completed")
	default:
		restful.Add(ws)
	}
}

// return a clone of the global configuration, minus any plugin-specific JSON
func (c *pluginContext) Config() *records.Config {
	cfg := c.config
	cfg.Plugins = nil
	cfg.Masters = make([]string, len(c.config.Masters))
	copy(cfg.Masters, c.config.Masters)
	cfg.Resolvers = make([]string, len(c.config.Resolvers))
	copy(cfg.Resolvers, c.config.Resolvers)
	return &cfg
}

// launch third-party plugins
func (c *pluginContext) initAddonPlugins() {
	for _, pconfig := range c.config.Plugins {
		pluginName := pconfig.Name
		if pluginName == "" {
			logging.Error.Printf("failed to register plugin with empty name")
			continue
		}
		plugin, err := plugins.New(pluginName, pconfig.Settings)
		if err != nil {
			logging.Error.Printf("failed to create plugin: %v", err)
			continue
		}
		c.launchPlugin(pluginName, plugin)
	}
}

func (c *pluginContext) launchPlugin(pluginName string, plugin plugins.Plugin) {
	logging.Verbose.Printf("starting plugin %q", pluginName)
	if errCh := plugin.Start(c); errCh != nil {
		go func() {
			for err := range errCh {
				c.errHandler(pluginName, err)
			}
		}()
	}
	go func() {
		//TODO(jdef) should plugins be restarted if they fail? might not apply to all plugin types.
		//could be mitigated via some plugin lifecycle type.
		select {
		case <-plugin.Done():
			logging.Verbose.Printf("plugin %q terminated", pluginName)
		}
	}()
}
