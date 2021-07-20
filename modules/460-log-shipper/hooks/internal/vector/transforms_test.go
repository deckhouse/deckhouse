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

		assert.Len(t, tr, 4)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[{"inputs":["testit"],"group_by":["file","stream"],"merge_strategies": {"message":"concat"}, "type": "reduce", "starts_when": " match!(.message, r'^Traceback|^[ ]+|(ERROR|INFO|DEBUG|WARN)') || match!(.message, r'^((([a-zA-Z\\-0-9]+)_([a-zA-Z\\-0-9]+)\\s)|(([a-zA-Z\\-0-9]+)\\s)|(.{0}))(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\.\\d{3}) \\[|^(\\{\\s{0,1}\")|^(\\d{2}-\\w{3}-\\d{4}\\s\\d{2}:\\d{2}:\\d{2}\\.{0,1}\\d{2,3})\\s(\\w+)|^([A-Z][0-9]{0,4}\\s\\d{2}:\\d{2}:\\d{2}\\.\\d{0,6})') || match!(.message, r'^[^\\s]') " },{"inputs":[ "d8_tf_testit_0" ],"source":" label1 = .pod_labels.\"controller-revision-hash\" \n if label1 != null { \n   del(.pod_labels.\"controller-revision-hash\") \n } \n label2 = .pod_labels.\"pod-template-hash\" \n if label2 != null { \n   del(.pod_labels.\"pod-template-hash\") \n } \n label3 = .kubernetes \n if label3 != null { \n   del(.kubernetes) \n } \n label4 = .file \n if label4 != null { \n   del(.file) \n } \n","type":"remap", "drop_on_abort": false},{"inputs":[ "d8_tf_testit_1" ],"source":" .foo=\"bar\" \n","type":"remap", "drop_on_abort": false},{"inputs":["d8_tf_testit_2"],"source":" structured, err1 = parse_json(.message) \n if err1 == null { \n   .data = structured \n   del(.message) \n } else { \n   .data.message = del(.message)\n } \n","type":"remap", "drop_on_abort": false}]`, string(data))
	})
}
