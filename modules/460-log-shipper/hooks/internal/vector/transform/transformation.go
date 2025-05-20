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

func BuildModes(transform v1alpha1.Transformation) []apis.LogTransform {
	transforms := make([]apis.LogTransform, 0)
	if transform.NormalizeLabelKeys {
		transforms = append(transforms, normalizeLabelKeys())
	}
	if transform.EnsureStructuredMessage.TargetField != "" {
		transforms = append(transforms, ensureStructuredMessage(transform.TargetField))
	}
	if len(transform.DropLabels.Labels) > 0 {
		transforms = append(transforms, dropLabels(transform.DropLabels.Labels))
	}
	return transforms
}

func normalizeLabelKeys() apis.LogTransform {
	name := "transformation_normalizeLabelKeys"
	vrl := "if exists(.pod_labels) {\n.pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}"
	return NewTransformation(name, vrl)
}

func ensureStructuredMessage(targetField string) apis.LogTransform {
	name := "transformation_ensureStructuredMessage"
	vrl := fmt.Sprintf(".message = parse_json(.message) ?? { \"%s\": .message }\n", targetField)
	return NewTransformation(name, vrl)
}

func dropLabels(labels []string) apis.LogTransform {
	var vrl string
	name := "transformation_dropLabels"
	ls := checkFixDotPrefix(labels)
	for _, l := range ls {
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
