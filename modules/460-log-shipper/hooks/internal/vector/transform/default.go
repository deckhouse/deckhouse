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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/model"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/vrl"
)

func CleanUpAfterSourceTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "clean_up",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.CleanUpAfterSourceRule.String(),
			"drop_on_abort": false,
		},
	}
}

func JSONParseTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "json_parse",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.ParseJSONRule.String(),
			"drop_on_abort": false,
		},
	}
}

func CreateDefaultTransforms(dest v1alpha1.ClusterLogDestination) []impl.LogTransform {
	transforms := []impl.LogTransform{
		CleanUpAfterSourceTransform(),
		JSONParseTransform(),
	}

	switch dest.Spec.Type {
	case model.DestElasticsearch, model.DestLogstash:
		transforms = append(transforms, DeDotTransform())

		if len(dest.Spec.ExtraLabels) > 0 {
			transforms = append(transforms, ExtraFieldTransform(dest.Spec.ExtraLabels))
		}
	}

	if dest.Spec.Type == model.DestElasticsearch && dest.Spec.Elasticsearch.DataStreamEnabled {
		transforms = append(transforms, DataStreamTransform())
	}

	if dest.Spec.RateLimit.LinesPerMinute != nil {
		transforms = append(transforms, ThrottleTransform(dest.Spec.RateLimit))
	}

	return transforms
}
