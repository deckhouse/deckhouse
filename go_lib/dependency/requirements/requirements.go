/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package requirements

import (
	"fmt"
	"strings"
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

func RegisterDisruptionFunc(key string, f DisruptionFunc) {
	once.Do(
		func() {
			defaultRegistry = newRegistry()
		},
	)

	defaultRegistry.RegisterDisruption(key, f)
}

// CheckRequirement run check function for `key` requirement. Returns true if check is passed, false otherwise
func CheckRequirement(key, value string, getter ValueGetter) (bool, error) {
	if defaultRegistry == nil {
		return true, nil
	}

	if strings.HasPrefix(key, "disruption:") {
		return true, nil
	}

	f, err := defaultRegistry.GetByKey(key)
	if err != nil {
		panic(err)
	}

	return f(value, getter)
}

func HasDisruption(key, _ string, _ ValueGetter) (bool, string) {
	if defaultRegistry == nil {
		return false, ""
	}

	if !strings.HasPrefix(key, "disruption:") {
		return false, ""
	}

	f, err := defaultRegistry.GetDisruptionByKey(key)
	if err != nil {
		return false, ""
	}

	return f()
}

type CheckFunc func(requirementValue string, getter ValueGetter) (bool, error)
type DisruptionFunc func() (bool, string)

type ValueGetter interface {
	Get(path string) gjson.Result
}

type requirementsResolver interface {
	Register(key string, f CheckFunc)
	RegisterDisruption(key string, f DisruptionFunc)
	GetByKey(key string) (CheckFunc, error)
	GetDisruptionByKey(key string) (DisruptionFunc, error)
}

type requirementsRegistry struct {
	checkers    map[string]CheckFunc
	disruptions map[string]DisruptionFunc
}

func newRegistry() *requirementsRegistry {
	return &requirementsRegistry{
		checkers:    make(map[string]CheckFunc),
		disruptions: make(map[string]DisruptionFunc),
	}
}

func (r *requirementsRegistry) Register(key string, f CheckFunc) {
	r.checkers[key] = f
}

func (r *requirementsRegistry) RegisterDisruption(key string, f DisruptionFunc) {
	r.disruptions[key] = f
}

func (r *requirementsRegistry) GetByKey(key string) (CheckFunc, error) {
	f, ok := r.checkers[key]
	if !ok {
		return nil, fmt.Errorf("check function for %q requirement is not registred", key)
	}

	return f, nil
}

func (r *requirementsRegistry) GetDisruptionByKey(key string) (DisruptionFunc, error) {
	f, ok := r.disruptions[key]
	if !ok {
		return nil, fmt.Errorf("disruption function for %q is not registred", key)
	}

	return f, nil
}
