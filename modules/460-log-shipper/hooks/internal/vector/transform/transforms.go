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
	"errors"
	"fmt"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
)

func BuildFromMapSlice(inputName string, trans []impl.LogTransform) ([]impl.LogTransform, error) {
	prevInput := inputName

	for i, trm := range trans {
		trm.SetName(fmt.Sprintf("d8_tf_%s_%02d_%s", inputName, i, trm.GetName()))
		trm.SetInputs([]string{prevInput})
		prevInput = trm.GetName()
		trans[i] = trm
	}

	return trans, nil
}

type CommonTransform struct {
	Name   string   `json:"-"`
	Type   string   `json:"type"`
	Inputs []string `json:"inputs"`
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
	cs.Inputs = inp
}

func (cs *CommonTransform) GetInputs() []string {
	return cs.Inputs
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

func (t *DynamicTransform) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// TODO(nabokihms): I fixed this function, but it previously did not work, yet results are the same in all tests
	var transformMap map[string]interface{}
	err := unmarshal(&transformMap)
	if err != nil {
		return err
	}

	tstr, ok := transformMap["type"].(string)
	if !ok {
		return errors.New("the `type` is required and it has to be of the string type")
	}

	inp, ok := transformMap["inputs"].([]string)
	if !ok {
		inp = make([]string, 0)
	}

	delete(transformMap, "inputs")
	delete(transformMap, "type")

	// nolint: ineffassign
	t = &DynamicTransform{
		CommonTransform: CommonTransform{
			Type:   tstr,
			Inputs: inp,
		},
		DynamicArgsMap: transformMap,
	}

	return nil
}
