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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
)

func TestGenerateParseMessageVRL(t *testing.T) {
	t.Run("JSON", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			spec v1alpha2.ParseMessageSpec
			want string
		}{
			{
				"root merges object",
				v1alpha2.ParseMessageSpec{SourceFormat: v1alpha2.FormatJSON, TargetLabel: ".", JSON: v1alpha2.SourceFormatJSONSpec{}},
				`if is_string(.message) {
  parsed = parse_json(.message) ?? null
  if parsed != null {

    if is_object(parsed) {
      . = merge!(., parsed, deep: true)
    }

  }
}`,
			},
			{
				"depth to .message",
				v1alpha2.ParseMessageSpec{SourceFormat: v1alpha2.FormatJSON, JSON: v1alpha2.SourceFormatJSONSpec{Depth: 2}},
				`if is_string(.message) {
  parsed = parse_json(.message, max_depth: 2) ?? null
  if parsed != null {

    . = set!(., ["message"], parsed)

  }
}`,
			},
			{
				"custom path",
				v1alpha2.ParseMessageSpec{SourceFormat: v1alpha2.FormatJSON, TargetLabel: ".foo.bar", JSON: v1alpha2.SourceFormatJSONSpec{}},
				`if is_string(.message) {
  parsed = parse_json(.message) ?? null
  if parsed != null {

    . = set!(., ["foo", "bar"], parsed)

  }
}`,
			},
			{
				"targetLabel parsed",
				v1alpha2.ParseMessageSpec{SourceFormat: v1alpha2.FormatJSON, TargetLabel: ".parsed", JSON: v1alpha2.SourceFormatJSONSpec{}},
				`if is_string(.message) {
  parsed = parse_json(.message) ?? null
  if parsed != null {

    . = set!(., ["parsed"], parsed)

  }
}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := GenerateParseMessageVRL(tc.spec)
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("unary", func(t *testing.T) {
		shell := `if is_string(.message) {
  parsed = %s ?? null
  if parsed != null {

    . = set!(., ["message"], parsed)

  }
}`
		for _, tc := range []struct {
			fmt  v1alpha2.SourceFormat
			expr string
		}{
			{v1alpha2.FormatKlog, "parse_klog(.message)"},
			{v1alpha2.FormatCLF, "parse_common_log(.message)"},
			{v1alpha2.FormatSysLog, "parse_syslog(.message)"},
			{v1alpha2.FormatLogfmt, "parse_logfmt(.message)"},
		} {
			got, err := GenerateParseMessageVRL(v1alpha2.ParseMessageSpec{SourceFormat: tc.fmt})
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf(shell, tc.expr), got, string(tc.fmt))
		}
	})

	t.Run("unary targetLabel parsed", func(t *testing.T) {
		got, err := GenerateParseMessageVRL(v1alpha2.ParseMessageSpec{SourceFormat: v1alpha2.FormatKlog, TargetLabel: ".parsed"})
		require.NoError(t, err)
		want := `if is_string(.message) {
  parsed = parse_klog(.message) ?? null
  if parsed != null {

    . = set!(., ["parsed"], parsed)

  }
}`
		assert.Equal(t, want, got)
	})

	t.Run("String regex", func(t *testing.T) {
		got, err := GenerateParseMessageVRL(v1alpha2.ParseMessageSpec{
			SourceFormat: v1alpha2.FormatString,
			String: v1alpha2.SourceFormatStringSpec{
				Regex: `^(\d+)$`,
				SetLabels: map[string]string{
					"z": "lit",
					"a": "{{ grp }}",
				},
			},
		})
		require.NoError(t, err)
		want := `if is_string(.message) {
  parsed, perr = parse_regex(string!(.message), r'^(\d+)$')
  if perr == null {
    out = {}
out = set!(out, ["a"], string!(parsed.grp))
out = set!(out, ["z"], "lit")

    . = set!(., ["message"], out)

  }
}`
		assert.Equal(t, want, got)
	})
}
