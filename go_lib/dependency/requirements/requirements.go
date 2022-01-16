package requirements

import (
	"fmt"
	"sync"

	"github.com/tidwall/gjson"
)

var (
	once            sync.Once
	defaultRegistry requirementsResolver
)

func Register(key string, f CheckFunc) {
	once.Do(
		func() {
			defaultRegistry = newRegistry()
		},
	)

	defaultRegistry.Register(key, f)
}

func CheckRequirement(key, value string, getter ValueGetter) (bool, error) {
	if defaultRegistry == nil {
		return true, nil
	}
	f, err := defaultRegistry.GetByKey(key)
	if err != nil {
		panic(err)
	}

	return f(value, getter)
}

type CheckFunc func(requirementValue string, getter ValueGetter) (bool, error)

type ValueGetter interface {
	Get(path string) gjson.Result
}

type requirementsResolver interface {
	Register(key string, f CheckFunc)
	GetByKey(key string) (CheckFunc, error)
}

type requirementsRegistry struct {
	funcs map[string]CheckFunc
}

func newRegistry() *requirementsRegistry {
	return &requirementsRegistry{
		funcs: make(map[string]CheckFunc),
	}
}

func (r *requirementsRegistry) Register(key string, f CheckFunc) {
	r.funcs[key] = f
}

func (r *requirementsRegistry) GetByKey(key string) (CheckFunc, error) {
	f, ok := r.funcs[key]
	if !ok {
		return nil, fmt.Errorf("check function for %q requirement is not registred", key)
	}

	return f, nil
}
