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

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

type LogSourceConfig struct {
	SourceType string

	MultilineType v1alpha1.MultiLineParserType

	LabelFilter []v1alpha1.Filter
	LogFilter   []v1alpha1.Filter
}

func CreateLogSourceTransforms(name string, cfg *LogSourceConfig) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)

	transforms = append(transforms, CreateMultiLineTransforms(cfg.MultilineType)...)

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

	sTransforms, err := BuildFromMapSlice(name, transforms)
	if err != nil {
		return nil, fmt.Errorf("add source transforms: %v", err)
	}

	return sTransforms, nil
}
