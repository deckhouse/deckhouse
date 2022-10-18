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

package transform

import (
	"fmt"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
)

func BuildFromMapSlice(prefix, inputName string, transforms []apis.LogTransform) ([]apis.LogTransform, error) {
	prevInput := ""

	for i, transform := range transforms {
		name := fmt.Sprintf("transform/%s/%s/%02d_%s", prefix, inputName, i, transform.GetName())

		transform.SetName(name)
		if prevInput != "" {
			transform.SetInputs([]string{prevInput})
		}

		prevInput = transform.GetName()
		transforms[i] = transform
	}

	return transforms, nil
}

type CommonTransform struct {
	Name   string  `json:"-"`
	Type   string  `json:"type"`
	Inputs set.Set `json:"inputs"`
}

func (cs *CommonTransform) GetName() string {
	if cs.Name == "" {
		return "unknown"
	}
	return cs.Name
}

func (cs *CommonTransform) SetName(name string) {
	cs.Name = name
}

func (cs *CommonTransform) SetInputs(inp []string) {
	cs.Inputs.Add(inp...)
}

func (cs *CommonTransform) GetInputs() []string {
	return cs.Inputs.Slice()
}

type DynamicTransform struct {
	CommonTransform

	DynamicArgsMap map[string]interface{} `json:"-"`
}

func (t *DynamicTransform) MarshalJSON() ([]byte, error) {
	// TODO(nabokihms): think about this hack
	type dt DynamicTransform // prevent recursion
	b, _ := json.Marshal(dt(*t))

	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)

	for k, v := range t.DynamicArgsMap {
		b, _ = json.Marshal(v)
		m[k] = b
	}

	return json.Marshal(m)
}
