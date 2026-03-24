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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation"
)

func BuildTransformations(tms []v1alpha1.TransformationSpec) ([]apis.LogTransform, []string, error) {
	transforms := []apis.LogTransform{}
	var addLabelsSinkKeys []string
	for _, tm := range tms {
		var lt apis.LogTransform
		switch tm.Action {
		case v1alpha1.AddLabels:
			source, keys, err := transformation.AddLabelsVRL(tm.AddLabels)
			if err != nil {
				return nil, nil, fmt.Errorf("transformations addLabels: %w", err)
			}
			addLabelsSinkKeys = append(addLabelsSinkKeys, keys...)
			lt = NewTransformation("tf_addLabels", source)
		case v1alpha1.ReplaceKeys:
			source, err := transformation.ReplaceKeysVRL(tm.ReplaceKeys)
			if err != nil {
				return nil, nil, fmt.Errorf("transformations replaceKeys: %w", err)
			}
			lt = NewTransformation("tf_replaceKeys", source)
		case v1alpha1.ParseMessage:
			vrlName := fmt.Sprintf("tf_parseMessage_%s", tm.ParseMessage.SourceFormat)
			source, err := transformation.GenerateParseMessageVRL(tm.ParseMessage)
			if err != nil {
				return nil, nil, fmt.Errorf("transformations parseMessage: %w", err)
			}
			lt = NewTransformation(vrlName, source)
		case v1alpha1.DropLabels:
			source, err := transformation.DropLabelsVRL(tm.DropLabels)
			if err != nil {
				return nil, nil, fmt.Errorf("transformations dropLabels: %w", err)
			}
			lt = NewTransformation("tf_dropLabels", source)
		case v1alpha1.ReplaceValue:
			source, err := transformation.ReplaceValueVRL(tm.ReplaceValue)
			if err != nil {
				return nil, nil, fmt.Errorf("transformations replaceValue: %w", err)
			}
			lt = NewTransformation("tf_replaceValue", source)
		default:
			return nil, nil, fmt.Errorf("transformations action: %q not valid", tm.Action)
		}
		if lt != nil {
			transforms = append(transforms, lt)
		}
	}
	return transforms, addLabelsSinkKeys, nil
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
