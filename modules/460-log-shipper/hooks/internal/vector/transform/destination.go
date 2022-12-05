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

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func CreateLogDestinationTransforms(name string, dest v1alpha1.ClusterLogDestination) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)

	switch dest.Spec.Type {
	case v1alpha1.DestElasticsearch, v1alpha1.DestLogstash:
		transforms = append(transforms, DeDotTransform())
		fallthrough
	case v1alpha1.DestVector, v1alpha1.DestKafka:
		if len(dest.Spec.ExtraLabels) > 0 {
			transforms = append(transforms, ExtraFieldTransform(dest.Spec.ExtraLabels))
		}
	}

	if dest.Spec.Type == v1alpha1.DestElasticsearch && dest.Spec.Elasticsearch.DataStreamEnabled {
		transforms = append(transforms, DataStreamTransform())
	}

	if dest.Spec.RateLimit.LinesPerMinute != nil {
		transforms = append(transforms, ThrottleTransform(dest.Spec.RateLimit))
	}

	switch dest.Spec.Type {
	case v1alpha1.DestElasticsearch, v1alpha1.DestLogstash, v1alpha1.DestVector:
		transforms = append(transforms, CleanUpParsedDataTransform())
	case v1alpha1.DestLoki:
		if len(dest.Spec.ExtraLabels) > 0 {
			transforms = append(transforms, CreateParseDataTransforms())
		}
	}

	dTransforms, err := BuildFromMapSlice("destination", name, transforms)
	if err != nil {
		return nil, fmt.Errorf("add source transforms: %v", err)
	}

	return dTransforms, nil
}
