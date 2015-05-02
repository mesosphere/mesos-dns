package plugins

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
)

type FakePluginConfig struct {
	Foo int `json:"foo,omitempty"`
}

type fakePlugin struct {
	FakePluginConfig
	startFunc func(Context)
	stopFunc  func()
	done      chan struct{}
	doneOnce  sync.Once
}

func (p *fakePlugin) Start(ctx Context) {
	if p.startFunc != nil {
		p.startFunc(ctx)
	}
}

func (p *fakePlugin) Stop() {
	p.doneOnce.Do(func() {
		close(p.done)
		if p.stopFunc != nil {
			p.stopFunc()
		}
	})
}

func (p *fakePlugin) Done() <-chan struct{} {
	return p.done
}

func TestPluginConfig(t *testing.T) {
	logging.SetupLogs()

	foo := 0
	Register("fake", Factory(func(raw json.RawMessage) (Plugin, error) {
		var c FakePluginConfig
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FakePluginConfig: %v", err)
		}
		return &fakePlugin{
			FakePluginConfig: c,
			done:             make(chan struct{}),
			startFunc: func(ctx Context) {
				foo = c.Foo
			},
		}, nil
	}))
	jsonData := `{"plugins":[{"name":"fake","settings":{"Foo":123}}]}`
	conf := records.ParseConfig([]byte(jsonData), records.Config{Masters: []string{"bar"}, HttpOn: true, SOAMname: "d.e", SOARname: "a@b.c"})

	var raw json.RawMessage
	found := false
	for _, pconfig := range conf.Plugins {
		if pconfig.Name == "fake" {
			found = true
			raw = pconfig.Settings
			break
		}
	}
	if !found {
		t.Fatalf("failed to locate fake plugin configuration")
	}

	p, err := New("fake", raw)
	if err != nil {
		t.Fatalf("failed to create fake plugin instance: %v", err)
	}

	p.Start(nil)
	if foo != 123 {
		t.Fatalf("plugin not started successfully")
	}
}
