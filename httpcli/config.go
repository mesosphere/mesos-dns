package httpcli

import (
	"errors"
	"net/http"
	"sync"
)

// AuthMechanism enumerates the supported authentication strategies
type AuthMechanism string

// ErrDuplicateAuthRegistration signifies a configuration error in which the same
// AuthMechanism is being registered multiple times.
var (
	ErrDuplicateAuthRegistration        = errors.New("duplicate auth mechanism registration")
	ErrUnregisteredFactory              = errors.New("unregistered factory requested")
	ErrIllegalDoerFactoryImplementation = errors.New("illegal DoerFactory implementation returned non-nil Doer for a nil *http.Client")
	ErrMissingConfiguration             = errors.New("missing configuration for specified authentication mechanism")
)

// AuthNone, et al. represent the complete set of supported authentication mechanisms
const (
	AuthNone  AuthMechanism = ""      // AuthNone specifies no authentication mechanism
	AuthBasic               = "basic" // AuthBasic specifies to use HTTP Basic
	AuthIAM                 = "iam"   // AuthIAM specifies to use IAM / JDK authentication
)

var (
	defaultFactoriesState = map[AuthMechanism]DoerFactory{
		AuthNone: DoerFactory(func(_ ConfigMap, client *http.Client) (doer Doer) {
			if client != nil {
				doer = client
			}
			return
		}),
	}

	registry = struct {
		sync.Mutex
		factories map[AuthMechanism]DoerFactory
	}{factories: defaultFactories()}
)

func defaultFactories() (m map[AuthMechanism]DoerFactory) {
	m = make(map[AuthMechanism]DoerFactory)
	for k, v := range defaultFactoriesState {
		m[k] = v
	}
	return
}

// ConfigMap maps authentication configuration types to values
type ConfigMap map[AuthMechanism]interface{}

// ConfigMapOption is a functional option for a ConfigMap
type ConfigMapOption func(ConfigMap)

// ConfigMapOptions aggregates ConfigMapOption
type ConfigMapOptions []ConfigMapOption

// ToConfigMap generates a ConfigMap from the given options. If no map entries are generated
// then a nil ConfigMap is returned.
func (cmo ConfigMapOptions) ToConfigMap() (m ConfigMap) {
	m = make(ConfigMap)
	for _, opt := range cmo {
		if opt != nil {
			opt(m)
		}
	}
	if len(m) == 0 {
		m = nil
	}
	return m
}

// FindOrPanic returns the mapped configuration for the given auth mechanism or else panics
func (cm ConfigMap) FindOrPanic(am AuthMechanism) interface{} {
	obj, ok := cm[am]
	if !ok {
		panic(ErrMissingConfiguration)
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

// RegistryReset unregisters all previously user-registered factory instances and resets to the
// default state. Intended for testing purposes.
func RegistryReset() {
	registry.Lock()
	defer registry.Unlock()
	registry.factories = defaultFactories()
}

func factoryFor(am AuthMechanism) (df DoerFactory, ok bool) {
	registry.Lock()
	defer registry.Unlock()
	df, ok = registry.factories[am]
	return
}

// Validate checks that the given AuthMechainsm and ConfigMap are compatible with the
// registered set of DoerFactory instances.
func Validate(am AuthMechanism, cm ConfigMap) (err error) {
	df, ok := factoryFor(am)
	if !ok {
		err = ErrUnregisteredFactory
	} else {
		defer func() {
			// recover from a factory panic
			if v := recover(); v != nil {
				if verr, ok := v.(error); ok {
					err = verr
				} else {
					panic(v) // unexpected, forward this up the stack
				}
			}
		}()
		if shouldBeNil := df(cm, nil); shouldBeNil != nil {
			err = ErrIllegalDoerFactoryImplementation
		}
	}
	return
}
