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
	in   v1alpha1.Transform
	out  string
}{
	{"fixNestedJson label message", v1alpha1.Transform{Action: "EnsureStructuredMessage", TargetField: "text"},
		".message = parse_json(.message) ?? { \"text\": .message }\n"},
	{"del", v1alpha1.Transform{Action: "DropLabels", Labels: []string{"first", "second"}},
		"if exists(.first) {\n del(.first)\n}\nif exists(.second) {\n del(.second)\n}\n"},
	// {"delZero", v1alpha1.Transform{Action: "DropLabels", Labels: []string{}}, ""},
	{"replaceDot", v1alpha1.Transform{Action: "NormalizeLabelKeys"},
		"if exists(.pod_labels) {\n.pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\")}\n}"},
}

func TestReplaceDot(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			tr, err := BuildModes([]v1alpha1.Transform{test.in})
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, tr, 1)
			transform := tr[0].(*DynamicTransform)
			assert.Equal(t, test.out, transform.DynamicArgsMap["source"].(string))
		})
	}
}
