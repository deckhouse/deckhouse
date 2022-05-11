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

package handler

import (
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transform"
)

type transformApplier struct {
	destination   v1alpha1.ClusterLogDestination
	multilineType v1alpha1.MultiLineParserType

	labelFilter []v1alpha1.Filter
	logFilter   []v1alpha1.Filter
}

func (t *transformApplier) Do(transforms []impl.LogTransform) ([]impl.LogTransform, error) {
	transforms = append(transforms, transform.CreateMultiLineTransforms(t.multilineType)...)
	transforms = append(transforms, transform.CreateDefaultTransforms(t.destination)...)

	labelFilterTransforms, err := transform.CreateLabelFilterTransforms(t.labelFilter)
	if err != nil {
		return nil, err
	}

	transforms = append(transforms, labelFilterTransforms...)
	logFilterTransforms, err := transform.CreateLogFilterTransforms(t.logFilter)
	if err != nil {
		return nil, err
	}

	transforms = append(transforms, logFilterTransforms...)
	transforms = append(transforms, transform.CreateDefaultCleanUpTransforms(t.destination)...)

	return transforms, nil
}
