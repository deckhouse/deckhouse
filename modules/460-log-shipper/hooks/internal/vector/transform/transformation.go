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

func BuildModes(tms []v1alpha1.TransformationSpec) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var transformation apis.LogTransform
	var err error
	for num, tm := range tms {
		switch tm.Action {
		case "ReplaceDotKeys":
			if len(tm.ReplaceDotKeys.Labels) > 0 {
				transformation = replaceDotKeys(num, tm.ReplaceDotKeys)
			}
		case "EnsureStructuredMessage":
			if tm.EnsureStructuredMessage.SourceFormat != "" {
				transformation, err = ensureStructuredMessage(num, tm.EnsureStructuredMessage)
				if err != nil {
					return nil, err
				}
			}
		case "DropLabels":
			if len(tm.DropLabels.Labels) > 0 {
				transformation = dropLabels(num, tm.DropLabels)
			}
		default:
			return nil, fmt.Errorf("transformions: action %s not valide", tm.Action)
		}
		transforms = append(transforms, transformation)
	}
	return transforms, nil
}

func replaceDotKeys(num int, r v1alpha1.ReplaceDotKeysSpec) apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_replaceDotKeys_%d", num)
	labels := checkFixDotPrefix(r.Labels)
	for _, l := range labels {
		vrl = fmt.Sprintf("if exists(%s) {\n%s = map_keys(object!(%s), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}", l, l, l)
	}
	return NewTransformation(name, vrl)
}

func ensureStructuredMessage(num int, e v1alpha1.EnsureStructuredMessageSpec) (apis.LogTransform, error) {
	var vrl string
	name := fmt.Sprintf("tf_ensureStructuredMessage_%s_%d", e.SourceFormat, num)
	switch e.SourceFormat {
	case "String":
		if e.String.TargetField == "" {
			return nil, fmt.Errorf("transformions ensureStructuredMessage string: TargetField is empty")
		}
		vrl = fmt.Sprintf(".message = parse_json(.message) ?? { \"%s\": .message }\n", e.String.TargetField)
	case "JSON":
		if e.JSON.Depth == 0 {
			return nil, fmt.Errorf("transformions ensureStructuredMessage JSON: Depth is empty")
		}
		vrl = fmt.Sprintf(".message = parse_json!(.message, max_depth: %d)\n", e.JSON.Depth)
	case "Klog":
		vrl = ".message = parse_json(.message) ?? parse_klog!(.message)\n"
	default:
		return nil, fmt.Errorf("transformions ensureStructuredMessage: sourceFormat %s not valide", e.SourceFormat)
	}
	return NewTransformation(name, vrl), nil
}

func dropLabels(num int, d v1alpha1.DropLabelsSpec) apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_dropLabels_%d", num)
	labels := checkFixDotPrefix(d.Labels)
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
