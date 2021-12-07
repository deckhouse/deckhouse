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
					"app": "{{ app }}",
				},
			},
		}

		defaultTransforms := CreateDefaultTransforms(dest)

		transforms = append(transforms, defaultTransforms...)
		transforms = append(transforms, CreateDefaultCleanUpTransforms(dest)...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 5)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"inputs":[ "testit" ],"source":" if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\")\n }\n  if exists(.pod_labels.\"pod-template-hash\") {\n   del(.pod_labels.\"pod-template-hash\")\n }\n if exists(.kubernetes) {\n   del(.kubernetes)\n }\n if exists(.file) {\n   del(.file)\n }\n","type":"remap", "drop_on_abort": false},{"inputs":["d8_tf_testit_0"],"source":" structured, err1 = parse_json(.message)\n if err1 == null {\n   .parsed_data = structured\n }\n","type":"remap", "drop_on_abort": false},{"hooks": {"process":"process"}, "inputs":["d8_tf_testit_1"], "source":"\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n", "type":"lua", "version":"2"},{"inputs":[ "d8_tf_testit_2" ],"source":" if exists(.parsed_data.app) { .app=.parsed_data.app } \n .foo=\"bar\" \n","type":"remap", "drop_on_abort": false},{"inputs":[ "d8_tf_testit_3" ],"source":" if exists(.parsed_data) {\n   del(.parsed_data)\n }\n","type":"remap", "drop_on_abort": false}]`, string(data))
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

		assert.JSONEq(t, `[{"condition":"exists(.parsed_data.info)", "inputs":["testit"], "type":"filter"}, {"condition":"if is_boolean(.parsed_data.severity) || is_float(.parsed_data.severity) { data, err = to_string(.parsed_data.severity); if err != null { false; } else { includes([\"aaa\",42], data); }; } else { includes([\"aaa\",42], .parsed_data.severity); }", "inputs":["d8_tf_testit_0"], "type":"filter"}]`, string(data))
	})

	t.Run("Test extra labels", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)
		extraLabels := make(map[string]string)
		extraLabels["aba"] = "bbb"
		extraLabels["aaa"] = "{{ pay-load[0].a }}"
		extraLabels["aca"] = "{{ test.pay\\.lo\\.ad.hel\\.lo.world }}"
		extraLabels["add"] = "{{ test.pay\\.lo }}"
		extraLabels["adc"] = "{{ pay\\.lo.test }}"
		extraLabels["bdc"] = "{{ pay\\.lo[3].te\\.st }}"
		extraFieldsTransform := GenExtraFieldsTransform(extraLabels)
		transforms = append(transforms, &extraFieldsTransform)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"inputs":["testit"], "type":"remap", "drop_on_abort": false, "source": " if exists(.parsed_data.\"pay-load\"[0].a) { .aaa=.parsed_data.\"pay-load\"[0].a } \n .aba=\"bbb\" \n if exists(.parsed_data.test.\"pay.lo.ad\".\"hel.lo\".world) { .aca=.parsed_data.test.\"pay.lo.ad\".\"hel.lo\".world } \n if exists(.parsed_data.\"pay.lo\".test) { .adc=.parsed_data.\"pay.lo\".test } \n if exists(.parsed_data.test.\"pay.lo\") { .add=.parsed_data.test.\"pay.lo\" } \n if exists(.parsed_data.\"pay.lo\"[3].\"te.st\") { .bdc=.parsed_data.\"pay.lo\"[3].\"te.st\" } \n"}]`, string(data))
	})

	t.Run("Test multiline 1", func(t *testing.T) {
		multilineTransforms := CreateMultiLinaeTransforms(v1alpha1.MultiLineParserNone)
		transforms := make([]impl.LogTransform, 0)
		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 0)

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[]`, string(data))
	})

	t.Run("Test multiline 2", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)

		multilineTransforms := CreateMultiLinaeTransforms(v1alpha1.MultiLineParserGeneral)

		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"group_by":["file", "stream"], "inputs":["testit"], "merge_strategies":{"message":"concat"}, "starts_when":" if exists(.message) { if length(.message) > 0 { matched, err = match(.message, r'^[^\\s\\t]'); if err != null { false; } else { matched; }; } else { false; }; } else { false; } ", "type":"reduce"}]`, string(data))
	})
}
