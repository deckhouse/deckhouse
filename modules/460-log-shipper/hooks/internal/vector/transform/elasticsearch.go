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

// DeDotTransform is the only Lua transform rule.
// We are going to replace it with corresponding VRL transform once the iteration feature will be implemented for VRL.
// Related issue https://github.com/timberio/vector/issues/3588
func DeDotTransform() *DynamicTransform {
	const deDotSnippet = `
function process(event, emit)
	if event.log.pod_labels == nil then
		return
	end
	dedot(event.log.pod_labels)
	emit(event)
end
function dedot(map)
	if map == nil then
		return
	end
	local new_map = {}
	local changed_keys = {}
	for k, v in pairs(map) do
		local dedotted = string.gsub(k, "%.", "_")
		if dedotted ~= k then
			new_map[dedotted] = v
			changed_keys[k] = true
		end
	end
	for k in pairs(changed_keys) do
		map[k] = nil
	end
	for k, v in pairs(new_map) do
		map[k] = v
	end
end`
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "elastic_dedot",
			Type: "lua",
		},
		DynamicArgsMap: map[string]interface{}{
			"version": "2",
			"hooks": map[string]interface{}{
				"process": "process",
			},
			"source": deDotSnippet,
		},
	}
}

func DataStreamTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "elastic-stream",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.StreamRule.String(),
			"drop_on_abort": false,
		},
	}
}

func CleanUpParsedDataTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "del_parsed_data",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.ParsedDataCleanUpRule.String(),
			"drop_on_abort": false,
		},
	}
}

func CreateDefaultCleanUpTransforms(dest v1alpha1.ClusterLogDestination) []impl.LogTransform {
	transforms := make([]impl.LogTransform, 0)
	switch dest.Spec.Type {
	case model.DestElasticsearch, model.DestLogstash:
		transforms = append(transforms, CleanUpParsedDataTransform())
	}
	return transforms
}
