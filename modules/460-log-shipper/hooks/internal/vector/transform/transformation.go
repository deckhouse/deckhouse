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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

var (
	vectorLabelTemplate = regexp.MustCompile(`^\.[a-zA-Z0-9_\[\]\\\.\-]+$`)
)

func BuildModes(tms []v1alpha1.TransformationSpec) ([]apis.LogTransform, error) {
	transforms := []apis.LogTransform{}
	for _, tm := range tms {
		var err error
		var transformation apis.LogTransform
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
	sources := []string{}
	vrlName := "tf_replaceDotKeys"
	for _, l := range r.Labels {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformations replaceDotKeys label: %s not valid", l)
		}
		sources = append(sources, vrl.ReplaceDotKeys(l))
	}
	return NewTransformation(vrlName, strings.Join(sources, "\n")), nil
}

func ensureStructuredMessage(e v1alpha1.EnsureStructuredMessageSpec) (apis.LogTransform, error) {
	var source string
	vrlName := fmt.Sprintf("tf_ensureStructuredMessage_%s", e.SourceFormat)
	switch e.SourceFormat {
	case v1alpha1.FormatString:
		if e.String.TargetField == "" {
			return nil, fmt.Errorf("transformations ensureStructuredMessage string: TargetField is empty")
		}
		source = vrl.EnsureStructuredMessageString(e.String.TargetField)
	case v1alpha1.FormatJSON:
		source = vrl.EnsureStructuredMessageJSON(e.JSON.Depth)
	case v1alpha1.FormatKlog:
		source = vrl.EnsureStructuredMessageKlog()
	default:
		return nil, fmt.Errorf("transformations ensureStructuredMessage: sourceFormat %s not valid", e.SourceFormat)
	}
	return NewTransformation(vrlName, source), nil
}

func dropLabels(d v1alpha1.DropLabelsSpec) (apis.LogTransform, error) {
	sources := []string{}
	vrlName := "tf_dropLabels"
	for _, l := range d.Labels {
		if !validLabel(l) {
			return nil, fmt.Errorf("transformations dropLabels label: %s not valid", l)
		}
		sources = append(sources, vrl.DropLabels(l))
	}
	return NewTransformation(vrlName, strings.Join(sources, "\n")), nil
}

func NewTransformation(name, source string) *DynamicTransform {
	if source == "" || name == "" {
		return nil
	}
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   name,
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]any{
			"source":        source,
			"drop_on_abort": false,
		},
	}
}

func validLabel(label string) bool {
	return vectorLabelTemplate.MatchString(label)
}
