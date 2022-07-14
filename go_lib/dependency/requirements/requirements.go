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

const (
	DisruptionPrefix = "disruption:"
)

// RegisterCheck set CheckFunc for some component
func RegisterCheck(key string, f CheckFunc) {
	once.Do(
		func() {
			defaultRegistry = newRegistry()
		},
	)

	defaultRegistry.RegisterCheck(key, f)
}

// RegisterDisruptionFunc add DisruptionFunc for some component
func RegisterDisruption(key string, f DisruptionFunc) {
	once.Do(
		func() {
			defaultRegistry = newRegistry()
		},
	)

	if !strings.HasPrefix(key, DisruptionPrefix) {
		key = DisruptionPrefix + key
	}

	defaultRegistry.RegisterDisruption(key, f)
}

// CheckRequirement run check function for `key` requirement. Returns true if check is passed, false otherwise
func CheckRequirement(key, value string, getter ValueGetter) (bool, error) {
	if defaultRegistry == nil {
		return true, nil
	}

	if strings.HasPrefix(key, DisruptionPrefix) {
		return true, nil
	}

	f, err := defaultRegistry.GetCheckByKey(key)
	if err != nil {
		panic(err)
	}

	return f(value, getter)
}

// HasDisruption run check function for `key` disruption. Returns true if disruption condition is met, false otherwise. Returns reason for true response.
func HasDisruption(key, _ string, _ ValueGetter) (bool, string) {
	if defaultRegistry == nil {
		return false, ""
	}

	if !strings.HasPrefix(key, DisruptionPrefix) {
		return false, ""
	}

	f, err := defaultRegistry.GetDisruptionByKey(key)
	if err != nil {
		return false, ""
	}

	return f()
}

// CheckFunc check come precondition, comparing desired value (requirementValue) with current value (getter)
type CheckFunc func(requirementValue string, getter ValueGetter) (bool, error)
type DisruptionFunc func() (bool, string)

type ValueGetter interface {
	Get(path string) gjson.Result
}

type requirementsResolver interface {
	RegisterCheck(key string, f CheckFunc)
	GetCheckByKey(key string) (CheckFunc, error)

	RegisterDisruption(key string, f DisruptionFunc)
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

func (r *requirementsRegistry) RegisterCheck(key string, f CheckFunc) {
	r.checkers[key] = f
}

func (r *requirementsRegistry) RegisterDisruption(key string, f DisruptionFunc) {
	r.disruptions[key] = f
}

func (r *requirementsRegistry) GetCheckByKey(key string) (CheckFunc, error) {
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
