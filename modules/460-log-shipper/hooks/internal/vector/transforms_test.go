/*
Copyright 2021 Flant CJSC

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

package vector

import (
	"testing"

	"github.com/clarketm/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

func TestTransformSnippet(t *testing.T) {
	t.Run("marshal Transform", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)
		dest := v1alpha1.ClusterLogDestination{
			Spec: v1alpha1.ClusterLogDestinationSpec{
				Type: DestElasticsearch,
				ExtraLabels: map[string]string{
					"foo": "bar",
				},
			},
		}

		defaultTransforms := CreateDefaultTransforms(dest)

		transforms = append(transforms, defaultTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 5)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"inputs":["testit"],"group_by":["file","stream"],"merge_strategies": {"message":"concat"}, "type": "reduce", "starts_when": " match!(.message, r'^Traceback|^[ ]+|(ERROR|INFO|DEBUG|WARN)') || match!(.message, r'^((([a-zA-Z\\-0-9]+)_([a-zA-Z\\-0-9]+)\\s)|(([a-zA-Z\\-0-9]+)\\s)|(.{0}))(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\.\\d{3}) \\[|^(\\{\\s{0,1}\")|^(\\d{2}-\\w{3}-\\d{4}\\s\\d{2}:\\d{2}:\\d{2}\\.{0,1}\\d{2,3})\\s(\\w+)|^([A-Z][0-9]{0,4}\\s\\d{2}:\\d{2}:\\d{2}\\.\\d{0,6})') || match!(.message, r'^[^\\s]') " },{"inputs":[ "d8_tf_testit_0" ],"source":" label1 = .pod_labels.\"controller-revision-hash\" \n if label1 != null { \n   del(.pod_labels.\"controller-revision-hash\") \n } \n label2 = .pod_labels.\"pod-template-hash\" \n if label2 != null { \n   del(.pod_labels.\"pod-template-hash\") \n } \n label3 = .kubernetes \n if label3 != null { \n   del(.kubernetes) \n } \n label4 = .file \n if label4 != null { \n   del(.file) \n } \n","type":"remap", "drop_on_abort": false},{"inputs":[ "d8_tf_testit_1" ],"source":" .foo=\"bar\" \n","type":"remap", "drop_on_abort": false},{"hooks": {"process":"process"}, "inputs":["d8_tf_testit_2"], "source":"\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n", "type":"lua", "version":"2"},{"inputs":["d8_tf_testit_3"],"source":" structured, err1 = parse_json(.message) \n if err1 == null { \n   .data = structured \n   del(.message) \n } else { \n   .data.message = del(.message)\n } \n","type":"remap", "drop_on_abort": false}]`, string(data))
	})

	t.Run("Test filters", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)

		filters := make([]v1alpha1.LogFilter, 0)
		filters = append(filters, v1alpha1.LogFilter{
			Field:    "info",
			Operator: v1alpha1.LogFilterOpExists,
		})

		filters = append(filters, v1alpha1.LogFilter{
			Field:    "severity",
			Operator: v1alpha1.LogFilterOpIn,
			Values: []interface{}{
				"aaa",
				42,
			},
		})

		filterTransforms, _ := CreateTransformsFromFilter(filters)

		transforms = append(transforms, filterTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 2)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"condition":"exists(.data.info)", "inputs":["testit"], "type":"filter"}, {"condition":"if is_boolean(.data.severity) || is_float(.data.severity)\n { data, err = to_string(.data.severity)\n if err != null {\n false\n } else {\n includes([\"aaa\",42], data)\n } }\n else\n {\n includes([\"aaa\",42], .data.severity)\n }", "inputs":["d8_tf_testit_0"], "type":"filter"}]`, string(data))
	})
}
