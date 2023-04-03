/*
Copyright 2022 Flant JSC

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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func OwnerReferenceSourceTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "owner_ref",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.OwnerReferenceRule.String(),
			"drop_on_abort": false,
		},
	}
}

func CleanUpAfterSourceTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "clean_up",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.CleanUpAfterSourceRule.String(),
			"drop_on_abort": false,
		},
	}
}

type LogSourceConfig struct {
	SourceType string

	MultilineType         v1alpha1.MultiLineParserType
	MultilineCustomConfig v1alpha1.MultilineParserCustom
	LabelFilter           []v1alpha1.Filter
	LogFilter             []v1alpha1.Filter
}

func CreateLogSourceTransforms(name string, cfg *LogSourceConfig) ([]apis.LogTransform, error) {
	var transforms []apis.LogTransform

	if cfg.SourceType == v1alpha1.SourceKubernetesPods {
		transforms = append(transforms, OwnerReferenceSourceTransform())
	}

	transforms = append(transforms, CleanUpAfterSourceTransform())

	multilineTransforms, err := CreateMultiLineTransforms(cfg.MultilineType, cfg.MultilineCustomConfig)
	if err != nil {
		return nil, fmt.Errorf("error rendering multi line transforms: %v", err)
	}

	transforms = append(transforms, multilineTransforms...)

	labelFilterTransforms, err := CreateLabelFilterTransforms(cfg.LabelFilter)
	if err != nil {
		return nil, err
	}
	transforms = append(transforms, labelFilterTransforms...)

	logFilterTransforms, err := CreateLogFilterTransforms(cfg.LogFilter)
	if err != nil {
		return nil, err
	}
	transforms = append(transforms, logFilterTransforms...)

	sTransforms, err := BuildFromMapSlice("source", name, transforms)
	if err != nil {
		return nil, fmt.Errorf("add source transforms: %v", err)
	}

	return sTransforms, nil
}
