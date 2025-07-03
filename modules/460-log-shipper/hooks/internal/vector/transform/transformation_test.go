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
	{"ParseMessage String Format",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "String",
				String:       v1alpha1.SourceFormatStringSpec{TargetField: "text"},
			},
		},
		"if is_string(.message) {\n  .message = {\"text\": .message}\n}",
	},
	{"ParseMessage JSON Format ",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "JSON",
				JSON:         v1alpha1.SourceFormatJSONSpec{Depth: 1},
			},
		},
		`if is_string(.message) {
  .message = parse_json(
    .message, max_depth: 1
  ) ?? .message
}`,
	},
	{"ParseMessage Klog Format",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "Klog",
			},
		},
		"if is_string(.message) {\n  .message = parse_klog(.message) ?? .message\n}",
	},
	{"ParseMessage SysLog Format",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "SysLog",
			},
		},
		"if is_string(.message) {\n  .message = parse_syslog(.message) ?? .message\n}",
	},
	{"ParseMessage Logfmt Format",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "Logfmt",
			},
		},
		"if is_string(.message) {\n  .message = parse_logfmt(.message) ?? .message\n}",
	},
	{"ParseMessage CLF Format",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ParseMessage,
			ParseMessage: v1alpha1.ParseMessageSpec{
				SourceFormat: "CLF",
			},
		},
		"if is_string(.message) {\n  .message = parse_common_log(.message) ?? .message\n}",
	},
	{"DropLabels",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.DropLabels,
			DropLabels: v1alpha1.DropLabelsSpec{
				Labels: []string{".first", ".second"},
			},
		},
		"if exists(.first) {\n  del(.first)\n}\n" +
			"if exists(.second) {\n  del(.second)\n}",
	},
	{"ReplaceKeys",
		v1alpha1.TransformationSpec{
			Action: v1alpha1.ReplaceKeys,
			ReplaceKeys: v1alpha1.ReplaceKeysSpec{
				Source: ".",
				Target: "_",
				Labels: []string{".pod_labels", ".examples"},
			},
		},
		`if exists(.pod_labels) {
  .pod_labels = map_keys(
    object!(.pod_labels), recursive: true
  ) -> |key| {
    replace(key, ".", "_")
  }
}
if exists(.examples) {
  .examples = map_keys(
    object!(.examples), recursive: true
  ) -> |key| {
    replace(key, ".", "_")
  }
}`,
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
