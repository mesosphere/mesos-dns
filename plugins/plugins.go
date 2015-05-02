package plugins

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/emicklei/go-restful"
	"github.com/mesosphere/mesos-dns/resolver"
)

// A plugin has a single use lifecycle: once started, it may be stopped. once stopped,
// it may not be restarted.
type Plugin interface {
	// Performs initialization and starts any requisite background tasks for the plugin.
	// Pre-server-startup actions such as Filter or resolver.Reloader registration must
	// be completed before this func returns. This func is not expected to block for long
	// and should return relatively quickly.
	Start(Context)
	// Stops any running background tasks for the plugin, should return immediately.
	Stop()
	// Returns a signal chan that's closed once the plugin has terminated.
	Done() <-chan struct{}
}

// Build a new instance of a Plugin given some JSON configuration data.
type Factory func(json.RawMessage) (Plugin, error)

// Plugins use this interface to communicate with the underlying DNS resolver.
type Resolver interface {
	OnReload(r resolver.Reloader)
}

type Context interface {
	// Return a pointer to the mesos-dns Resolver.
	Resolver() Resolver

	// Adds a new filter handle some kind of pre- or post-processing of
	// DNS requests and/or responses.
	AddFilter(Filter)

	// Register the given plugin REST web service
	RegisterWS(*restful.WebService)

	// Return a signal chan that closes when the server enters shutdown mode.
	Done() <-chan struct{}
}

var (
	pluginsLock sync.Mutex
	plugins     = map[string]Factory{}
)

// Register a new Factory implementation for the given plugin name. Both fields
// are required and name is not allowed to be empty. It's recommended that names
// use a domain/label convention, for example "mesos-dns.io/fancyPlugin".
func Register(name string, f Factory) error {
	if name == "" {
		return fmt.Errorf("illegal plugin name")
	}
	if f == nil {
		return fmt.Errorf("nil Factory not allowed")
	}

	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	if _, found := plugins[name]; found {
		return fmt.Errorf("Factory for %q is already registered", name)
	}

	plugins[name] = f
	return nil
}

// Create a new plugin for the registered name and the given JSON configuration.
// Will return an error if either the name is unregistered or else if the registered
// factory generates an error attempting to build a new plugin instance.
func New(name string, raw json.RawMessage) (Plugin, error) {
	factory, found := func() (f Factory, ok bool) {
		pluginsLock.Lock()
		defer pluginsLock.Unlock()
		f, ok = plugins[name]
		return
	}()
	if !found {
		return nil, fmt.Errorf("no Factory registered for plugin name %q", name)
	}
	return factory(raw)
}
