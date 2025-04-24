package transform

import (
	"testing"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
)

var testCases = []struct {
	name string
	in   v1alpha1.TransformMod
	out  string
}{
	{"fixNestedJson lable message", v1alpha1.TransformMod{Action: "fixNestedJson", Label: "example"},
		".example = parse_json(.example) ?? { \"text\": .example }\n"},
	{"fixNestedJson lable with dot", v1alpha1.TransformMod{Action: "fixNestedJson", Label: ".example"},
		".example = parse_json(.example) ?? { \"text\": .example }\n"},
	{"fixNestedJson without label", v1alpha1.TransformMod{Action: "fixNestedJson", Label: ""},
		".message = parse_json(.message) ?? { \"text\": .message }\n"},
	{"del", v1alpha1.TransformMod{Action: "del", Label: "example"}, "del(.example)\n"},
	{"replaceDot", v1alpha1.TransformMod{Action: "replaceDot", Label: "example"},
		"if exists(.pod_labels) {\n    .pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, \".\", \"_\") }\n}"},
}

func TestReplaceDot(t *testing.T) {
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			tr, err := BuildModes([]v1alpha1.TransformMod{test.in})
			if err != nil {
				t.Error(err)
			}
			assert.Len(t, tr, 1)
			transform := tr[0].(*DynamicTransform)
			assert.Equal(t, test.out, transform.DynamicArgsMap["source"].(string))
		})
	}
}
