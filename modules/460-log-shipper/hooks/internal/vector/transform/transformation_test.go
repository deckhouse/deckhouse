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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

var testCases = []struct {
	name string
	in   v1alpha1.TransformationSpec
	out  string
}{
	{"EnsureStructuredMessage String Format Depth 1",
		v1alpha1.TransformationSpec{
			Action: "EnsureStructuredMessage",
			EnsureStructuredMessage: v1alpha1.EnsureStructuredMessageSpec{
				SourceFormat: "String",
				String:       v1alpha1.SourceFormatStringSpec{TargetField: "text", Depth: 1},
			},
		},
		".message = parse_json(.message, max_depth: 1) ?? { \"text\": .message }\n",
	},
	{"EnsureStructuredMessage String Format",
		v1alpha1.TransformationSpec{
			Action: "EnsureStructuredMessage",
			EnsureStructuredMessage: v1alpha1.EnsureStructuredMessageSpec{
				SourceFormat: "String",
				String:       v1alpha1.SourceFormatStringSpec{TargetField: "text"},
			},
		},
		".message = parse_json(.message) ?? { \"text\": .message }\n",
	},
	{"EnsureStructuredMessage JSON Format ",
		v1alpha1.TransformationSpec{
			Action: "EnsureStructuredMessage",
			EnsureStructuredMessage: v1alpha1.EnsureStructuredMessageSpec{
				SourceFormat: "JSON",
				JSON:         v1alpha1.SourceFormatJSONSpec{Depth: 1},
			},
		},
		".message = parse_json!(.message, max_depth: 1)\n",
	},
	{"EnsureStructuredMessage Klog Format",
		v1alpha1.TransformationSpec{
			Action: "EnsureStructuredMessage",
			EnsureStructuredMessage: v1alpha1.EnsureStructuredMessageSpec{
				SourceFormat: "Klog",
			},
		},
		".message = parse_json(.message) ?? parse_klog!(.message)\n",
	},
	{"DropLabels",
		v1alpha1.TransformationSpec{
			Action: "DropLabels",
			DropLabels: v1alpha1.DropLabelsSpec{
				Labels: []string{"first", "second"},
			},
		},
		"if exists(.first) {\n del(.first)\n}\n" +
			"if exists(.second) {\n del(.second)\n}\n",
	},
	{"ReplaceDotKeys",
		v1alpha1.TransformationSpec{
			Action: "ReplaceDotKeys",
			ReplaceDotKeys: v1alpha1.ReplaceDotKeysSpec{
				Labels: []string{"pod_labels", "examples"},
			},
		},
		"if exists(.pod_labels) {\n" +
			".pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}\n" +
			"if exists(.examples) {\n" +
			".examples = map_keys(object!(.examples), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}\n",
	},
}

func TestReplaceDot(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			tr, err := BuildModes([]v1alpha1.TransformationSpec{test.in})
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, tr, 1)
			transform := tr[0].(*DynamicTransform)
			assert.Equal(t, test.out, transform.DynamicArgsMap["source"].(string))
		})
	}
}
