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
				`val_0, err_0 = get(., ["ns"])
b_0 = false
if err_0 == null {
  if is_array(val_0) {
    hit_0 = filter(array!(val_0)) -> |_idx_0, el_0| {
      s_el_0, err_el_0 = to_string(el_0)
      err_el_0 == null && s_el_0 == "prod"
    }
    b_0 = length(hit_0) > 0
  } else {
    s_0, err_str_0 = to_string(val_0)
    b_0 = err_str_0 == null && s_0 == "prod"
  }
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
				`val_0, err_0 = get(., ["k"])
b_0 = false
if err_0 == null {
  if is_array(val_0) {
    hit_0 = filter(array!(val_0)) -> |_idx_0, el_0| {
      s_el_0, err_el_0 = to_string(el_0)
      err_el_0 == null && s_el_0 == "v"
    }
    b_0 = length(hit_0) == 0
  } else {
    s_0, err_str_0 = to_string(val_0)
    b_0 = err_str_0 == null && s_0 != "v"
  }
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
				`val_0, err_0 = get(., ["msg"])
b_0 = false
if err_0 == null {
  if is_array(val_0) {
    hit_0 = filter(array!(val_0)) -> |_idx_0, el_0| {
      s_el_0, err_el_0 = to_string(el_0)
      if err_el_0 != null {
        false
      } else {
        _, perr_0 = parse_regex(s_el_0, r'^[a-z]+$')
        perr_0 == null
      }
    }
    b_0 = length(hit_0) > 0
  } else {
    s_0, err_s_0 = to_string(val_0)
    if err_s_0 == null {
      _, perr_0 = parse_regex(s_0, r'^[a-z]+$')
      b_0 = perr_0 == null
    }
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
				`_, err_0 = get(., ["pod_labels", "app"])
b_0 = err_0 == null
if b_0 {
.tag = "x"
}`,
			},
			{
				"notexists",
				`!.pod_labels.skip`,
				map[string]string{".tag": "x"},
				`_, err_0 = get(., ["pod_labels", "skip"])
b_0 = err_0 != null
if b_0 {
.tag = "x"
}`,
			},
			{
				"!=~",
				`.msg !=~ '^\d+$'`,
				map[string]string{".ok": "1", ".lbl": "{{ .label }}"},
				`val_0, err_0 = get(., ["msg"])
b_0 = false
if err_0 == null {
  if is_array(val_0) {
    hit_0 = filter(array!(val_0)) -> |_idx_0, el_0| {
      s_el_0, err_el_0 = to_string(el_0)
      if err_el_0 != null {
        false
      } else {
        _, perr_0 = parse_regex(s_el_0, r'^\d+$')
        perr_0 == null
      }
    }
    b_0 = length(hit_0) == 0
  } else {
    s_0, err_s_0 = to_string(val_0)
    if err_s_0 == null {
      _, perr_0 = parse_regex(s_0, r'^\d+$')
      b_0 = perr_0 != null
    }
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
		assert.Equal(t, `val_0, err_0 = get(., ["ns"])
b_0 = false
if err_0 == null {
  if is_array(val_0) {
    hit_0 = filter(array!(val_0)) -> |_idx_0, el_0| {
      s_el_0, err_el_0 = to_string(el_0)
      err_el_0 == null && s_el_0 == "prod"
    }
    b_0 = length(hit_0) > 0
  } else {
    s_0, err_str_0 = to_string(val_0)
    b_0 = err_str_0 == null && s_0 == "prod"
  }
}
val_1, err_1 = get(., ["bar"])
b_1 = false
if err_1 == null {
  if is_array(val_1) {
    hit_1 = filter(array!(val_1)) -> |_idx_1, el_1| {
      s_el_1, err_el_1 = to_string(el_1)
      if err_el_1 != null {
        false
      } else {
        _, perr_1 = parse_regex(s_el_1, r'bar.*')
        perr_1 == null
      }
    }
    b_1 = length(hit_1) > 0
  } else {
    s_1, err_s_1 = to_string(val_1)
    if err_s_1 == null {
      _, perr_1 = parse_regex(s_1, r'bar.*')
      b_1 = perr_1 == null
    }
  }
}
val_2, err_2 = get(., ["tst"])
b_2 = false
if err_2 == null {
  if is_array(val_2) {
    hit_2 = filter(array!(val_2)) -> |_idx_2, el_2| {
      s_el_2, err_el_2 = to_string(el_2)
      err_el_2 == null && s_el_2 == "far"
    }
    b_2 = length(hit_2) == 0
  } else {
    s_2, err_str_2 = to_string(val_2)
    b_2 = err_str_2 == null && s_2 != "far"
  }
}
if b_0 && b_1 && b_2 {
.tag = "x"
}`, got)
	})
}
