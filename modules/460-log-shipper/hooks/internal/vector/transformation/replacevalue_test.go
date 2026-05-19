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

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
)

func TestReplaceValueVRL(t *testing.T) {
	t.Run("literal target", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: "secret",
			Target: "[REDACTED]",
			Labels: []string{".message"},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["message"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    replaced, rep_err = replace(value_str, r'secret', "[REDACTED]")
    if rep_err == null {
      . = set!(., ["message"], replaced)
    }
  }
}`, got)
	})

	t.Run("nested path", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: `key\d+`,
			Target: "X",
			Labels: []string{`.message.secret`},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["message", "secret"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    replaced, rep_err = replace(value_str, r'key\d+', "X")
    if rep_err == null {
      . = set!(., ["message", "secret"], replaced)
    }
  }
}`, got)
	})

	t.Run("multiple labels", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: "a",
			Target: "b",
			Labels: []string{".first", ".second"},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["first"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    replaced, rep_err = replace(value_str, r'a', "b")
    if rep_err == null {
      . = set!(., ["first"], replaced)
    }
  }
}
value, err = get(., ["second"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    replaced, rep_err = replace(value_str, r'a', "b")
    if rep_err == null {
      . = set!(., ["second"], replaced)
    }
  }
}`, got)
	})

	t.Run("named group target only", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: `(?P<uid>[0-9]+)`,
			Target: `{{ uid }}`,
			Labels: []string{".msg"},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["msg"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    parsed, perr = parse_regex(value_str, r'(?P<uid>[0-9]+)')
    if perr == null {
      replaced = replace(value_str, r'(?P<uid>[0-9]+)', string!(parsed.uid))
      . = set!(., ["msg"], replaced)
    }
  }
}`, got)
	})

	t.Run("named group with literal prefix and suffix", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: `(?P<code>\w+)`,
			Target: `ERR: {{ code }} done`,
			Labels: []string{".line"},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["line"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    parsed, perr = parse_regex(value_str, r'(?P<code>\w+)')
    if perr == null {
      replaced = replace(value_str, r'(?P<code>\w+)', "ERR: " + string!(parsed.code) + " done")
      . = set!(., ["line"], replaced)
    }
  }
}`, got)
	})

	t.Run("three named groups in target", func(t *testing.T) {
		got, err := ReplaceValueVRL(v1alpha2.ReplaceValueSpec{
			Source: `(?P<a>\w+)-(?P<b>\d+)-(?P<c>\w+)`,
			Target: `{{ a }}:{{ b }}:{{ c }}`,
			Labels: []string{".line"},
		})
		require.NoError(t, err)
		assert.Equal(t, `value, err = get(., ["line"])
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
    parsed, perr = parse_regex(value_str, r'(?P<a>\w+)-(?P<b>\d+)-(?P<c>\w+)')
    if perr == null {
      replaced = replace(value_str, r'(?P<a>\w+)-(?P<b>\d+)-(?P<c>\w+)', string!(parsed.a) + ":" + string!(parsed.b) + ":" + string!(parsed.c))
      . = set!(., ["line"], replaced)
    }
  }
}`, got)
	})

}
