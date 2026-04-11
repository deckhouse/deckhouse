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

package transformation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func TestAddLabelsVRL(t *testing.T) {
	t.Run("literals and path template", func(t *testing.T) {
		got, keys, err := AddLabelsVRL(v1alpha1.AddLabelsRule{
			SetLabels: map[string]string{".z": "1", ".a": "2"},
		})
		require.NoError(t, err)
		assert.Equal(t, ".a = \"2\"\n.z = \"1\"", got)
		assert.Equal(t, []string{"a", "z"}, keys)

		got, keys, err = AddLabelsVRL(v1alpha1.AddLabelsRule{
			SetLabels: map[string]string{".out": "{{ .src }}"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"out"}, keys)
		assert.Equal(t, `v, err = get(., ["src"])
if err == null {
  .out = v
}`, got)
	})

	t.Run("when", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			when   string
			labels map[string]string
			want   string
		}{
			{
				"==",
				`.ns == "prod"`,
				map[string]string{".tag": "x", ".lbl": "{{ .label }}"},
				`val, err = get(., ["ns"])
b_0 = false
if err == null {
  s, err_s = to_string(val)
  b_0 = err_s == null && s == "prod"
}
if b_0 {
v, err = get(., ["label"])
if err == null {
  .lbl = v
}
.tag = "x"
}`,
			},
			{
				"!=",
				`.k != 'v'`,
				map[string]string{".a": "1", ".lbl": "{{ .label }}"},
				`val, err = get(., ["k"])
b_0 = false
if err == null {
  s, err_s = to_string(val)
  b_0 = err_s == null && s != "v"
}
if b_0 {
.a = "1"
v, err = get(., ["label"])
if err == null {
  .lbl = v
}
}`,
			},
			{
				"=~",
				`.msg =~ '^[a-z]+$'`,
				map[string]string{".ok": "1", ".lbl": "{{ .label }}"},
				`val, err = get(., ["msg"])
b_0 = false
if err == null {
  s, err_s = to_string(val)
  if err_s == null {
    _, perr = parse_regex(s, r'^[a-z]+$')
    b_0 = perr == null
  }
}
if b_0 {
v, err = get(., ["label"])
if err == null {
  .lbl = v
}
.ok = "1"
}`,
			},
			{
				"exists",
				`.pod_labels.app`,
				map[string]string{".tag": "x"},
				`_, err = get(., ["pod_labels", "app"])
b_0 = err == null
if b_0 {
.tag = "x"
}`,
			},
			{
				"notexists",
				`!.pod_labels.skip`,
				map[string]string{".tag": "x"},
				`_, err = get(., ["pod_labels", "skip"])
b_0 = err != null
if b_0 {
.tag = "x"
}`,
			},
			{
				"!=~",
				`.msg !=~ '^\d+$'`,
				map[string]string{".ok": "1", ".lbl": "{{ .label }}"},
				`val, err = get(., ["msg"])
b_0 = false
if err == null {
  s, err_s = to_string(val)
  if err_s == null {
    _, perr = parse_regex(s, r'^\d+$')
    b_0 = perr != null
  }
}
if b_0 {
v, err = get(., ["label"])
if err == null {
  .lbl = v
}
.ok = "1"
}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, _, err := AddLabelsVRL(v1alpha1.AddLabelsRule{
					When:      []string{tc.when},
					SetLabels: tc.labels,
				})
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		for _, spec := range []v1alpha1.AddLabelsRule{
			{SetLabels: map[string]string{}},
			{When: []string{`broken`}, SetLabels: map[string]string{".a": "1"}},
			{When: []string{`.x =~ '['`}, SetLabels: map[string]string{".a": "1"}},
			{
				When:      []string{`.msg =~ 'x'`},
				SetLabels: map[string]string{".ok": "{{ lvl }}"},
			},
			{
				When:      []string{`.a == "{{ .b }}"`},
				SetLabels: map[string]string{".x": "1"},
			},
		} {
			_, _, err := AddLabelsVRL(spec)
			assert.Error(t, err)
		}
	})

	t.Run("when multiple AND", func(t *testing.T) {
		got, _, err := AddLabelsVRL(v1alpha1.AddLabelsRule{
			When: []string{
				`.ns == "prod"`,
				`.bar =~ "bar.*"`,
				`.tst != "far"`,
			},
			SetLabels: map[string]string{".tag": "x"},
		})
		require.NoError(t, err)
		assert.Equal(t, `val, err = get(., ["ns"])
b_0 = false
if err == null {
  s, err_s = to_string(val)
  b_0 = err_s == null && s == "prod"
}
val, err = get(., ["bar"])
b_1 = false
if err == null {
  s, err_s = to_string(val)
  if err_s == null {
    _, perr = parse_regex(s, r'bar.*')
    b_1 = perr == null
  }
}
val, err = get(., ["tst"])
b_2 = false
if err == null {
  s, err_s = to_string(val)
  b_2 = err_s == null && s != "far"
}
if b_0 && b_1 && b_2 {
.tag = "x"
}`, got)
	})
}
