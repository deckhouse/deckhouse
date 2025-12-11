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

// CreateLogDestinationTransforms creates a list of transforms for a log destination
func CreateLogDestinationTransforms(name string, dest v1alpha1.ClusterLogDestination) ([]apis.LogTransform, error) {
	var transforms []apis.LogTransform
	if dest.Spec.RateLimit.LinesPerMinute != nil {
		throttleTransform, err := ThrottleTransform(dest.Spec.RateLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to build throttle transform: %w", err)
		}
		transforms = append(transforms, throttleTransform)
	}
	if len(dest.Spec.Transformations) > 0 {
		customTransforms, err := BuildModes(dest.Spec.Transformations)
		if err != nil {
			return nil, fmt.Errorf("failed to build custom transformations: %w", err)
		}
		transforms = append(transforms, customTransforms...)
	}
	if len(dest.Spec.ExtraLabels) > 0 {
		transforms = append(transforms, ExtraFieldTransform(dest.Spec.ExtraLabels))
	}
	switch dest.Spec.Type {
	case v1alpha1.DestElasticsearch:
		transforms = append(transforms, DeDotTransform())
		if dest.Spec.Elasticsearch.DataStreamEnabled {
			transforms = append(transforms, DataStreamTransform())
		}
	case v1alpha1.DestLogstash:
		transforms = append(transforms, DeDotTransform())
	case v1alpha1.DestSocket:
		switch dest.Spec.Socket.Encoding.Codec {
		case v1alpha1.EncodingCodecSyslog:
			transforms = append(transforms, SyslogEncoding())
		case v1alpha1.EncodingCodecGELF:
			transforms = append(transforms, GELFCodecRelabeling())
		case v1alpha1.EncodingCodecCEF:
			transforms = append(transforms, CEFNameAndSeverity())
		}
	case v1alpha1.DestKafka:
		if dest.Spec.Kafka.Encoding.Codec == v1alpha1.EncodingCodecCEF {
			transforms = append(transforms, CEFNameAndSeverity())
		}
	case v1alpha1.DestSplunk:
		transforms = append(transforms, DateTime())
	case v1alpha1.DestVector:
	case v1alpha1.DestLoki:
	}
	transforms = append(transforms, CleanUpParsedDataTransform())
	dTransforms, err := BuildFromMapSlice("destination", name, transforms)
	if err != nil {
		return nil, fmt.Errorf("failed to build destination transforms: %w", err)
	}
	return dTransforms, nil
}
