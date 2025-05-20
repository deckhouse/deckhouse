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
	getTransform() apis.LogTransform
}

func BuildModes(tms []v1alpha1.Transform) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var module module
	for _, tm := range tms {
		switch tm.Action {
		case "NormalizeLabelKeys":
			module = normalizeLabelKeys{}
		case "EnsureStructuredMessage":
			module = ensureStructuredMessage{targetField: tm.TargetField}
		case "DropLabels":
			if len(tm.Labels) > 0 {
				module = dropLabels{labels: tm.Labels}
			}
		default:
			return nil, fmt.Errorf("TransformMod: action %s not found", tm.Action)
		}
		lofTransform := module.getTransform()
		transforms = append(transforms, lofTransform)
	}
	return transforms, nil
}

type normalizeLabelKeys struct{}

func (nlk normalizeLabelKeys) getTransform() apis.LogTransform {
	var vrl string
	name := "tf_normolazeLabelKeys"
	vrl = "if exists(.pod_labels) {\n.pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}"
	return NewTransformation(name, vrl)
}

type ensureStructuredMessage struct {
	targetField string
}

func (esm ensureStructuredMessage) getTransform() apis.LogTransform {
	var vrl string
	name := "tf_ensureStructuredMessage"
	vrl = fmt.Sprintf(".message = parse_json(.message) ?? { \"%s\": .message }\n", esm.targetField)
	return NewTransformation(name, vrl)
}

type dropLabels struct {
	labels []string
}

func (d dropLabels) getTransform() apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_delete_%s", splitAndRemoveDot(d.labels))
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

// name transform couldn't have dot
func splitAndRemoveDot(labels []string) string {
	var s string
	for _, l := range labels {
		l = strings.ReplaceAll(l, ".", "")
		s = fmt.Sprintf("%s_%s", s, l)
	}
	return s
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
