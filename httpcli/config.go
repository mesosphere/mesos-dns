package httpcli

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// AuthMechanism enumerates the supported authentication strategies
type AuthMechanism string

// ErrDuplicateAuthRegistration signifies a configuration error in which the same
// AuthMechanism is being registered multiple times.
var ErrDuplicateAuthRegistration = errors.New("duplicate auth mechanism registration")

// AuthNone, et al. represent the complete set of supported authentication mechanisms
const (
	AuthNone  AuthMechanism = ""      // AuthNone specifies no authentication mechanism
	AuthBasic               = "basic" // AuthBasic specifies to use HTTP Basic
	AuthIAM                 = "iam"   // AuthIAM specifies to use IAM / JDK authentication
)

var registry = struct {
	sync.Mutex
	factories map[AuthMechanism]DoerFactory
}{
	factories: map[AuthMechanism]DoerFactory{
		AuthNone: DoerFactory(func(_ ConfigMap, client *http.Client) Doer { return client }),
	},
}

// ConfigMap maps authentication configuration types to values
type ConfigMap map[AuthMechanism]interface{}

// FindOrPanic returns the mapped configuration for the given auth mechanism or else panics
func (cm ConfigMap) FindOrPanic(am AuthMechanism) interface{} {
	obj, ok := cm[am]
	if !ok {
		panic(fmt.Sprintf("missing configuration for auth mechanism %q", am))
	}
	return obj
}

// Register associates an AuthMechanism with a DoerFactory
func Register(am AuthMechanism, df DoerFactory) {
	registry.Lock()
	defer registry.Unlock()

	if _, ok := registry.factories[am]; ok {
		panic(ErrDuplicateAuthRegistration)
	}
	registry.factories[am] = df
}

func factoryFor(am AuthMechanism) (df DoerFactory, ok bool) {
	registry.Lock()
	defer registry.Unlock()
	df, ok = registry.factories[am]
	return
}
