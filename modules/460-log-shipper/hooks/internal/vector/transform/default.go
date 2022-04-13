package transform

import (
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/model"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/vrl"
)

func CleanUpTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "clean_up",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.CleanUpRule.String(),
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
		CleanUpTransform(),
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

	return transforms
}
