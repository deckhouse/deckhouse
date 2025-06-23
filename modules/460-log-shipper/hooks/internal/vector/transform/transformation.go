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
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

var (
	vectorLabelTemplate = regexp.MustCompile(`^[a-zA-Z0-9_\\\.\-]+$`)
)

func BuildModes(tms []v1alpha1.TransformationSpec) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var transformation apis.LogTransform
	var err error
	for _, tm := range tms {
		switch tm.Action {
		case "ReplaceDotKeys":
			if len(tm.ReplaceDotKeys.Labels) > 0 {
				transformation, err = replaceDotKeys(tm.ReplaceDotKeys)
				if err != nil {
					return nil, err
				}
			}
		case "EnsureStructuredMessage":
			if tm.EnsureStructuredMessage.SourceFormat != "" {
				transformation, err = ensureStructuredMessage(tm.EnsureStructuredMessage)
				if err != nil {
					return nil, err
				}
			}
		case "DropLabels":
			if len(tm.DropLabels.Labels) > 0 {
				transformation, err = dropLabels(tm.DropLabels)
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("transformions: action %s not valide", tm.Action)
		}
		transforms = append(transforms, transformation)
	}
	return transforms, nil
}

func replaceDotKeys(r v1alpha1.ReplaceDotKeysSpec) (apis.LogTransform, error) {
	var vrl string
	name := "tf_replaceDotKeys"
	labels := checkFixDotPrefix(r.Labels)
	for _, l := range labels {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformions replaceDotKeys label: %s not valide", l)
		}
		vrl = fmt.Sprintf("if exists(%s) {\n%s = map_keys(object!(%s), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}\n", l, l, l)
	}
	return NewTransformation(name, vrl), nil
}

func ensureStructuredMessage(e v1alpha1.EnsureStructuredMessageSpec) (apis.LogTransform, error) {
	var vrl string
	name := fmt.Sprintf("tf_ensureStructuredMessage_%s", e.SourceFormat)
	switch e.SourceFormat {
	case "String":
		if e.String.TargetField == "" {
			return nil, fmt.Errorf("transformions ensureStructuredMessage string: TargetField is empty")
		}
		vrl = fmt.Sprintf(".message = parse_json(.message) ?? { \"%s\": .message }\n", e.String.TargetField)
		if e.String.Depth != 0 {
			vrl = fmt.Sprintf(".message = parse_json(.message, max_depth: %d) ?? { \"%s\": .message }\n", e.String.Depth, e.String.TargetField)
		}
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

func dropLabels(d v1alpha1.DropLabelsSpec) (apis.LogTransform, error) {
	var vrl string
	name := "tf_dropLabels"
	labels := checkFixDotPrefix(d.Labels)
	for _, l := range labels {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformions dropLabels label: %s not valide", l)
		}
		vrl = fmt.Sprintf("%sif exists(%s) {\n del(%s)\n}\n", vrl, l, l)
	}
	return NewTransformation(name, vrl), nil
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

func validLabel(label string) bool {
	return vectorLabelTemplate.MatchString(label)
}
