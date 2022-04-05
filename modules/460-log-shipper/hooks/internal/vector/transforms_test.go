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
	"os"
	"path/filepath"
	"testing"

	"github.com/clarketm/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

func loadMock(t *testing.T, parts ...string) string {
	content, err := os.ReadFile(filepath.Join(append([]string{"testdata"}, parts...)...))
	require.NoError(t, err)

	return string(content)
}

func TestTransformSnippet(t *testing.T) {
	t.Run("Marshal Transform", func(t *testing.T) {
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

		assert.JSONEq(t, loadMock(t, "transform", "transform-snippet.json"), string(data))
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

		assert.JSONEq(t, loadMock(t, "transform", "filters.json"), string(data))
	})

	t.Run("Test extra labels", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)
		extraLabels := map[string]string{
			"aba": "bbb",
			"aaa": `{{ pay-load[0].a }}`,
			"aca": `{{ test.pay\.lo\.ad.hel\.lo.world }}`,
			"add": `{{ test.pay\.lo }}`,
			"adc": `{{ pay\.lo.test }}`,
			"bdc": `{{ pay\.lo[3].te\.st }}`,
		}

		transforms = append(transforms, extraFieldTransform(extraLabels))

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, loadMock(t, "transform", "extra-labels.json"), string(data))
	})

	t.Run("Test multiline None", func(t *testing.T) {
		multilineTransforms := CreateMultiLineTransforms(v1alpha1.MultiLineParserNone)
		transforms := make([]impl.LogTransform, 0)
		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 0)

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[]`, string(data))
	})

	t.Run("Test multiline General", func(t *testing.T) {
		transforms := make([]impl.LogTransform, 0)

		multilineTransforms := CreateMultiLineTransforms(v1alpha1.MultiLineParserGeneral)

		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildTransformsFromMapSlice("testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Equal(t, (tr[0].GetInputs())[0], "testit")

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, loadMock(t, "transform", "multiline.json"), string(data))
	})
}
