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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
)

func buildTransformations(destSpec v1alpha1.ClusterLogDestinationSpec, sourceType string) ([]apis.LogTransform, loglabels.DestinationSinkLabelMaps, error) {
	transforms := make([]apis.LogTransform, 0, len(destSpec.Transformations))
	withPodLabels := destSpec.Type == v1alpha1.DestLoki
	labelKeys := loglabels.MergedSourceAndExtraLables(sourceType, destSpec.ExtraLabels, withPodLabels)
	for _, tm := range destSpec.Transformations {
		var lt apis.LogTransform
		switch tm.Action {
		case v1alpha1.AddLabels:
			source, keys, err := transformation.AddLabelsVRL(tm.AddLabels)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations addLabels: %w", err)
			}
			labelKeys = loglabels.AppendAddLables(labelKeys, keys)
			lt = newTransformation("tf_addLabels", source)
		case v1alpha1.ReplaceKeys:
			source, err := transformation.ReplaceKeysVRL(tm.ReplaceKeys)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations replaceKeys: %w", err)
			}
			lt = newTransformation("tf_replaceKeys", source)
		case v1alpha1.ParseMessage:
			vrlName := fmt.Sprintf("tf_parseMessage_%s", tm.ParseMessage.SourceFormat)
			source, err := transformation.GenerateParseMessageVRL(tm.ParseMessage)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations parseMessage: %w", err)
			}
			lt = newTransformation(vrlName, source)
		case v1alpha1.DropLabels:
			source, dropVRLPaths, err := transformation.DropLabelsVRL(tm.DropLabels)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations dropLabels: %w", err)
			}
			dropSinkKeys, err := parser.SinkKeysFromVRLPaths(dropVRLPaths)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations dropLabels: %w", err)
			}
			labelKeys = loglabels.RemoveDropLables(labelKeys, dropSinkKeys)
			lt = newTransformation("tf_dropLabels", source)
		case v1alpha1.ReplaceValue:
			source, err := transformation.ReplaceValueVRL(tm.ReplaceValue)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations replaceValue: %w", err)
			}
			lt = newTransformation("tf_replaceValue", source)
		default:
			return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("transformations action: %q not valid", tm.Action)
		}
		if lt != nil {
			transforms = append(transforms, lt)
		}
	}
	sinkLabelMaps := loglabels.BuildDestinationSinkLabelMaps(destSpec, loglabels.DestinationSinkBuild{
		SourceType:    sourceType,
		WithPodLabels: withPodLabels,
		Keys:          labelKeys,
		Length:        len(labelKeys),
	})
	return transforms, sinkLabelMaps, nil
}

func newTransformation(name, source string) *DynamicTransform {
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
