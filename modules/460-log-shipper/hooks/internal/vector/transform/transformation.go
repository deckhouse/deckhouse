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
		case v1alpha1.ReplaceDotKeys:
			transformation, err = replaceDotKeys(tm.ReplaceDotKeys)
		case v1alpha1.EnsureStructuredMessage:
			transformation, err = ensureStructuredMessage(tm.EnsureStructuredMessage)
		case v1alpha1.DropLabels:
			transformation, err = dropLabels(tm.DropLabels)
		default:
			return nil, fmt.Errorf("transformations action: %s not valid", tm.Action)
		}
		if err != nil {
			return nil, err
		}
		if transformation != nil {
			transforms = append(transforms, transformation)
		}
	}
	return transforms, nil
}

func replaceDotKeys(r v1alpha1.ReplaceDotKeysSpec) (apis.LogTransform, error) {
	var vrl string
	name := "tf_replaceDotKeys"
	for _, l := range checkFixDotPrefix(r.Labels) {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformations replaceDotKeys label: %s not valid", l)
		}
		vrl = fmt.Sprintf("%sif exists(%s) {\n%s = map_keys(object!(%s), recursive: true) "+
			"-> |key| { replace(key, \".\", \"_\")}\n}\n", vrl, l, l, l)
	}
	return NewTransformation(name, vrl), nil
}

func ensureStructuredMessage(e v1alpha1.EnsureStructuredMessageSpec) (apis.LogTransform, error) {
	var vrl string
	vrlName := fmt.Sprintf("tf_ensureStructuredMessage_%s", e.SourceFormat)
	switch e.SourceFormat {
	case v1alpha1.FormatString:
		if e.String.TargetField == "" {
			return nil, fmt.Errorf("transformations ensureStructuredMessage string: TargetField is empty")
		}
		vrl = fmt.Sprintf(".message = parse_json(.message%s) ?? { \"%s\": .message }\n",
			addMaxDepth(e.String.Depth), e.String.TargetField)
	case v1alpha1.FormatJSON:
		vrl = fmt.Sprintf(".message = parse_json!(.message%s)\n", addMaxDepth(e.JSON.Depth))
	case v1alpha1.FormatKlog:
		vrl = fmt.Sprintf(".message = parse_json(.message%s) ?? parse_klog!(.message)\n", addMaxDepth(e.Klog.Depth))
	default:
		return nil, fmt.Errorf("transformations ensureStructuredMessage: sourceFormat %s not valid", e.SourceFormat)
	}
	return NewTransformation(vrlName, vrl), nil
}

func dropLabels(d v1alpha1.DropLabelsSpec) (apis.LogTransform, error) {
	var vrl string
	vrlName := "tf_dropLabels"
	for _, l := range checkFixDotPrefix(d.Labels) {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformations dropLabels label: %s not valid", l)
		}
		vrl = fmt.Sprintf("%sif exists(%s) {\n del(%s)\n}\n", vrl, l, l)
	}
	return NewTransformation(vrlName, vrl), nil
}

func NewTransformation(name, vrl string) *DynamicTransform {
	if vrl == "" || name == "" {
		return nil
	}
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

func addMaxDepth(depth int) string {
	if depth > 0 {
		return fmt.Sprintf(", max_depth: %d", depth)
	}
	return ""
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
