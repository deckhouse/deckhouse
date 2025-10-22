/*
Copyright 2025 Flant JSC

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

package helpers

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

var (
	ErrNoValue = errors.New("value not found")
)

type valuesAccessor[TValue any] struct {
	input *go_hook.HookInput
	path  string
}

func (values valuesAccessor[TValue]) Set(value TValue) {
	values.input.Values.Set(values.path, value)
}

func (values valuesAccessor[TValue]) Get() TValue {
	value := values.input.Values.Get(values.path)

	var ret TValue
	if !value.IsObject() {
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)

	return ret
}

func (values valuesAccessor[TValue]) Clear() {
	values.input.Values.Remove(values.path)
}

type ValuesAccessor[TValue any] interface {
	Set(value TValue)
	Get() TValue
	Clear()
}

func NewValuesAccessor[TValue any](input *go_hook.HookInput, path string) ValuesAccessor[TValue] {
	return valuesAccessor[TValue]{
		input: input,
		path:  path,
	}
}

func GetValue[TValue any](input *go_hook.HookInput, path string) (TValue, error) {
	var ret TValue

	value := input.Values.Get(path)
	if !value.Exists() {
		return ret, ErrNoValue
	}

	err := json.Unmarshal([]byte(value.Raw), &ret)
	if err != nil {
		return ret, fmt.Errorf("cannot unmarshal value: %w", err)
	}

	return ret, nil
}
