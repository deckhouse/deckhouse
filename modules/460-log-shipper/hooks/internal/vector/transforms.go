/*
Copyright 2021 Flant CJSC

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

package vector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

// Create default transforms
func CreateDefaultTransforms(dest v1alpha1.ClusterLogDestination) (transforms []impl.LogTransform) {

	// default multiline transform
	var multiLineTransform DynamicTransform = DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "reduce",
		},
		DynamicArgsMap: map[string]interface{}{
			"group_by": []string{
				"file",
				"stream",
			},
			"merge_strategies": map[string]string{
				"message": "concat",
			},
			"starts_when": " match!(.message, r'^Traceback|^[ ]+|(ERROR|INFO|DEBUG|WARN)') || match!(.message, r'^((([a-zA-Z\\-0-9]+)_([a-zA-Z\\-0-9]+)\\s)|(([a-zA-Z\\-0-9]+)\\s)|(.{0}))(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\.\\d{3}) \\[|^(\\{\\s{0,1}\")|^(\\d{2}-\\w{3}-\\d{4}\\s\\d{2}:\\d{2}:\\d{2}\\.{0,1}\\d{2,3})\\s(\\w+)|^([A-Z][0-9]{0,4}\\s\\d{2}:\\d{2}:\\d{2}\\.\\d{0,6})') || match!(.message, r'^[^\\s]') ",
		},
	}

	// default cleanup transform
	var cleanUpTransform DynamicTransform = DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source": ` label1 = .pod_labels."controller-revision-hash" 
 if label1 != null { 
   del(.pod_labels."controller-revision-hash") 
 } 
 label2 = .pod_labels."pod-template-hash" 
 if label2 != null { 
   del(.pod_labels."pod-template-hash") 
 } 
 label3 = .kubernetes 
 if label3 != null { 
   del(.kubernetes) 
 } 
 label4 = .file 
 if label4 != null { 
   del(.file) 
 } 
`,
			"drop_on_abort": false,
		},
	}

	// default logstash & elasticsearch json parser transform
	var JSONParseTransform DynamicTransform = DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source": ` structured, err1 = parse_json(.message) 
 if err1 == null { 
   .data = structured 
   del(.message) 
 } else { 
   .data.message = del(.message)
 } 
`,
			"drop_on_abort": false,
		},
	}

	transforms = make([]impl.LogTransform, 0)
	// default multiline transform
	transforms = append(transforms, &multiLineTransform)
	// default cleanup transform
	transforms = append(transforms, &cleanUpTransform)
	// Adding specific storage transforms
	if dest.Spec.Type == DestElasticsearch || dest.Spec.Type == DestLogstash {
		if len(dest.Spec.ExtraLabels) > 0 {
			extraFieldsTransform := GenExtraFieldsTransform(dest.Spec.ExtraLabels)
			transforms = append(transforms, &extraFieldsTransform)
		}
		transforms = append(transforms, &JSONParseTransform)
	}

	return transforms
}

func BuildTransformsFromMapSlice(inputName string, trans []impl.LogTransform) ([]impl.LogTransform, error) {

	prevInput := inputName
	for i, trm := range trans {
		trm.SetName(fmt.Sprintf("d8_tf_%s_%d", inputName, i))
		trm.SetInputs([]string{prevInput})
		prevInput = trm.GetName()
		trans[i] = trm
	}

	return trans, nil
}

func GenExtraFieldsTransform(extraFields map[string]string) (transform DynamicTransform) {

	tmpFields := make([]string, 0, len(extraFields))
	for k, v := range extraFields {
		tmpFields = append(tmpFields, fmt.Sprintf(" .%s=\"%s\" \n", k, v))
	}

	extraFieldsTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        strings.Join(tmpFields, ""),
			"drop_on_abort": false,
		},
	}

	return extraFieldsTransform
}

func GenJSONParse() (transform map[string]map[string]interface{}) {

	cleanUpTransform := make(map[string]map[string]interface{})

	cleanUpTransform["remap"] = make(map[string]interface{})
	cleanUpTransform["remap"]["source"] = ` structured, err1 = parse_json(.message) 
 if err1 == null { 
   .data = structured 
   del(.message) 
 } else { 
   .data.message = del(.message)
 } 
`
	cleanUpTransform["remap"]["drop_on_abort"] = false

	return cleanUpTransform
}

type CommonTransform struct {
	Name   string   `json:"-"`
	Type   string   `json:"type"`
	Inputs []string `json:"inputs"`
}

func (cs *CommonTransform) GetName() string {
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

func (t DynamicTransform) MarshalJSON() ([]byte, error) {

	type dt DynamicTransform // prevent recursion
	b, _ := json.Marshal(dt(t))

	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)

	for k, v := range t.DynamicArgsMap {
		b, _ = json.Marshal(v)
		m[k] = b
	}

	return json.Marshal(m)
}

func (t DynamicTransform) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var transformMap map[string]interface{}
	err := unmarshal(&transformMap)
	if err != nil {
		return err
	}

	tstr, ok := transformMap["type"].(string)
	if !ok {
		return errors.New("type required and have to be string")
	}

	inp, ok := transformMap["inputs"].([]string)
	if !ok {
		inp = make([]string, 0)
	}

	delete(transformMap, "inputs")
	delete(transformMap, "type")

	newtr := DynamicTransform{
		CommonTransform: CommonTransform{
			Type:   tstr,
			Inputs: inp,
		},
		DynamicArgsMap: transformMap,
	}
	t = newtr

	return nil
}
