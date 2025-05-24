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
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

type module interface {
	getTransform(int) apis.LogTransform
}

func BuildModes(tms []v1alpha1.Transform) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var module module
	for i, tm := range tms {
		switch tm.Action {
		case "NormalizeLabelKeys":
			module = normalizeLabelKeys{}
		case "EnsureStructuredMessage":
			module = ensureStructuredMessage{targetField: tm.TargetField}
		case "DropLabels":
			if len(tm.Labels) == 0 {
				continue
			}
			module = dropLabels{labels: tm.Labels}

		default:
			return nil, fmt.Errorf("TransformMod: action %s not found", tm.Action)
		}
		lofTransform := module.getTransform(i)
		transforms = append(transforms, lofTransform)
	}
	return transforms, nil
}

type normalizeLabelKeys struct{}

func (nlk normalizeLabelKeys) getTransform(number int) apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_normalizeLabelKeys_%d", number)
	vrl = "if exists(.pod_labels) {\n.pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}"
	return NewTransformation(name, vrl)
}

type ensureStructuredMessage struct {
	targetField string
}

func (esm ensureStructuredMessage) getTransform(number int) apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_ensureStructuredMessage_%d", number)
	vrl = fmt.Sprintf(".message = parse_json(.message) ?? { \"%s\": .message, \"level\": \"info\", \"name\": \"\", \"time\": \"\"}\n", esm.targetField)
	return NewTransformation(name, vrl)
}

type dropLabels struct {
	labels []string
}

func (d dropLabels) getTransform(number int) apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_delete_%d", number)
	labels := checkFixDotPrefix(d.labels)
	for _, l := range labels {
		vrl = fmt.Sprintf("%sif exists(%s) {\n del(%s)\n}\n", vrl, l, l)
	}
	return NewTransformation(name, vrl)
}

func NewTransformation(name, vrl string) *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   name,
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]any{
			"source":        vrl,
			"drop_on_abort": false,
		},
	}
}

// dot in label prefix need for vector
func checkFixDotPrefix(lbs []string) []string {
	labels := []string{}
	for _, l := range lbs {
		if !strings.HasPrefix(l, ".") {
			labels = append(labels, fmt.Sprintf(".%s", l))
			continue
		}
		labels = append(labels, l)
	}
	return labels
}
