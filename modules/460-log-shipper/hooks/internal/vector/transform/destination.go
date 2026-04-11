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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
)

func CreateLogDestinationTransforms(name string, dest v1alpha1.ClusterLogDestination, sourceType string) ([]apis.LogTransform, loglabels.DestinationSinkLabelMaps, error) {
	var transforms []apis.LogTransform
	if dest.Spec.RateLimit.LinesPerMinute != nil {
		throttleTransform, err := throttleTransform(dest.Spec.RateLimit)
		if err != nil {
			return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("failed to build throttle transform: %w", err)
		}
		transforms = append(transforms, throttleTransform)
	}
	if len(dest.Spec.ExtraLabels) > 0 {
		transforms = append(transforms, extraFieldTransform(dest.Spec.ExtraLabels))
	}
	customTransforms, sinkLabelMaps, err := buildTransformations(dest.Spec, sourceType)
	if err != nil {
		return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("failed to build custom transformations: %w", err)
	}
	transforms = append(transforms, customTransforms...)
	switch dest.Spec.Type {
	case v1alpha1.DestElasticsearch:
		transforms = append(transforms, deDotTransform())
		if dest.Spec.Elasticsearch.DataStreamEnabled {
			transforms = append(transforms, dataStreamTransform())
		}
	case v1alpha1.DestLogstash:
		transforms = append(transforms, deDotTransform())
	case v1alpha1.DestSocket:
		switch dest.Spec.Socket.Encoding.Codec {
		case v1alpha1.EncodingCodecSyslog:
			syslogLabels, err := syslogLabelsTransform(sinkLabelMaps.SyslogStructuredDataKeys)
			if err != nil {
				return nil, loglabels.DestinationSinkLabelMaps{}, fmt.Errorf("syslog structured data labels: %w", err)
			}
			transforms = append(transforms, syslogLabels)
			transforms = append(transforms, syslogEncoding())
		case v1alpha1.EncodingCodecGELF:
			transforms = append(transforms, gelfCodecRelabeling())
		case v1alpha1.EncodingCodecCEF:
			transforms = append(transforms, cefNameAndSeverity())
		}
	case v1alpha1.DestKafka:
		if dest.Spec.Kafka.Encoding.Codec == v1alpha1.EncodingCodecCEF {
			transforms = append(transforms, cefNameAndSeverity())
		}
	case v1alpha1.DestSplunk:
		transforms = append(transforms, dateTime())
	case v1alpha1.DestVector:
	case v1alpha1.DestLoki:
	}
	transforms = append(transforms, cleanUpParsedDataTransform())
	destTransformBase := fmt.Sprintf("transform/%s/destination/%s", sourceType, name)
	return buildFromMapSlice(destTransformBase, transforms), sinkLabelMaps, nil
}
