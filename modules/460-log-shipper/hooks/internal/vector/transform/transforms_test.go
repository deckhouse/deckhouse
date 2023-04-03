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
	"os"
	"path/filepath"
	"testing"

	"github.com/clarketm/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func compareMock(t *testing.T, data []byte, parts ...string) {
	filename := filepath.Join(append([]string{"testdata"}, parts...)...)
	content, err := os.ReadFile(filename)
	require.NoError(t, err)

	if os.Getenv("D8_LOG_SHIPPER_SAVE_TESTDATA") == "yes" {
		err := os.WriteFile(filename, data, 0600)
		require.NoError(t, err)
	}

	assert.JSONEq(t, string(content), string(data))
}

func TestTransformSnippet(t *testing.T) {
	t.Run("Test filters", func(t *testing.T) {
		transforms := make([]apis.LogTransform, 0)

		filters := make([]v1alpha1.Filter, 0)
		filters = append(filters, v1alpha1.Filter{
			Field:    "info",
			Operator: v1alpha1.FilterOpExists,
		})

		filters = append(filters, v1alpha1.Filter{
			Field:    "severity",
			Operator: v1alpha1.FilterOpIn,
			Values: []interface{}{
				"aaa",
				42,
			},
		})

		filters = append(filters, v1alpha1.Filter{
			Field:    "namespace",
			Operator: v1alpha1.FilterOpRegex,
			Values: []interface{}{
				"d8-.*",
				"kube-.*",
			},
		})

		filters = append(filters, v1alpha1.Filter{
			Field:    "namespace",
			Operator: v1alpha1.FilterOpNotRegex,
			Values: []interface{}{
				"dev-.*",
				"prod-.*",
			},
		})

		filterTransforms, err := CreateLogFilterTransforms(filters)
		require.NoError(t, err)

		transforms = append(transforms, filterTransforms...)

		tr, err := BuildFromMapSlice("prefix", "testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 5)
		assert.Len(t, tr[0].GetInputs(), 0)

		data, err := json.MarshalIndent(tr, "", "\t")
		require.NoError(t, err)

		compareMock(t, data, "filters.json")
	})

	t.Run("Test extra labels", func(t *testing.T) {
		transforms := make([]apis.LogTransform, 0)
		extraLabels := map[string]string{
			"aba": "bbb",
			"aaa": `{{ pay-load[0].a }}`,
			"aca": `{{ test.pay\.lo\.ad.hel\.lo.world }}`,
			"add": `{{ test.pay\.lo }}`,
			"adc": `{{ pay\.lo.test }}`,
			"bdc": `{{ pay\.lo[3].te\.st }}`,
			"bda": `{{ pay\.lo[0].te-st }}`,
			"abd": `{{ pay-load[0].te\.st }}`,
			"dba": `{{ parsed_data }}`,
		}

		transforms = append(transforms, ExtraFieldTransform(extraLabels))

		tr, err := BuildFromMapSlice("prefix", "testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Len(t, tr[0].GetInputs(), 0)

		data, err := json.MarshalIndent(tr, "", "\t")
		require.NoError(t, err)

		compareMock(t, data, "extra-labels.json")
	})

	t.Run("Test multiline None", func(t *testing.T) {
		multilineTransforms, err := CreateMultiLineTransforms(v1alpha1.MultiLineParserNone, v1alpha1.MultilineParserCustom{})
		assert.NoError(t, err)

		transforms := make([]apis.LogTransform, 0)
		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildFromMapSlice("prefix", "testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 0)

		data, err := json.Marshal(tr)
		require.NoError(t, err)

		assert.JSONEq(t, `[]`, string(data))
	})

	t.Run("Test multiline General", func(t *testing.T) {
		transforms := make([]apis.LogTransform, 0)

		multilineTransforms, err := CreateMultiLineTransforms(v1alpha1.MultiLineParserGeneral, v1alpha1.MultilineParserCustom{})
		assert.NoError(t, err)

		transforms = append(transforms, multilineTransforms...)

		tr, err := BuildFromMapSlice("prefix", "testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 1)
		assert.Len(t, tr[0].GetInputs(), 0)

		data, err := json.MarshalIndent(tr, "", "\t")
		require.NoError(t, err)

		compareMock(t, data, "multiline.json")
	})

	t.Run("Test multiline custom", func(t *testing.T) {
		transforms := make([]apis.LogTransform, 0)

		customConfigs := []v1alpha1.MultilineParserCustom{
			{
				EndsWhen: &v1alpha1.ParserRegex{
					NotRegex: pointer.String("^endsWhenRegexEnd"),
				},
			},
			{
				StartsWhen: &v1alpha1.ParserRegex{
					NotRegex: pointer.String("^startsWhenRegexEnd"),
				},
			},
			{
				EndsWhen: &v1alpha1.ParserRegex{
					Regex: pointer.String("^endsWhenRegex"),
				},
			},
			{
				EndsWhen: &v1alpha1.ParserRegex{
					NotRegex: pointer.String("^startsWhenRegex"),
				},
			},
		}
		for _, cfg := range customConfigs {
			multilineTransforms, err := CreateMultiLineTransforms(v1alpha1.MultiLineParserCustom, cfg)
			assert.NoError(t, err)
			transforms = append(transforms, multilineTransforms...)
		}

		tr, err := BuildFromMapSlice("prefix", "testit", transforms)
		require.NoError(t, err)

		assert.Len(t, tr, 4)
		assert.Len(t, tr[0].GetInputs(), 0)

		data, err := json.MarshalIndent(tr, "", "\t")
		require.NoError(t, err)

		compareMock(t, data, "multiline-custom.json")
	})
}
