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

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWhen(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			in       string
			wantPath string
			wantOp   WhenOp
			wantVal  string
		}{
			{"exists_path_only", `.pod_labels.app`, ".pod_labels.app", WhenExists, ""},
			{"exists_trim", `  .namespace  `, ".namespace", WhenExists, ""},
			{"notexists", `!.pod_labels.app`, ".pod_labels.app", WhenNotExists, ""},
			{"notexists_trim", `  !.namespace  `, ".namespace", WhenNotExists, ""},
			{"eq_unquoted", `.pod == foo`, ".pod", WhenEQ, "foo"},
			{"eq_double", `.pod == "foo"`, ".pod", WhenEQ, "foo"},
			{"ne_single", `.pod  !=  'bar'`, ".pod", WhenNE, "bar"},
			{"=~", `.msg =~ ".*"`, ".msg", WhenRe, ".*"},
			{"=~_unquoted", `.msg =~ .*`, ".msg", WhenRe, ".*"},
			{"!=~", `.msg !=~ "[0-9]+"`, ".msg", WhenNRe, "[0-9]+"},
			{"nested", `.a.b !=~ 'x'`, ".a.b", WhenNRe, "x"},
			{"trim", ` .nested.path == "v" `, ".nested.path", WhenEQ, "v"},
			{"escaped_in_value", ".k == \"a\\\"b\"", ".k", WhenEQ, `a"b`},
			{"quoted_value_with_eq_inside", `.path =~ "x==y"`, ".path", WhenRe, "x==y"},
			{"value_with_double_amp", `.a == "x&&y"`, ".a", WhenEQ, "x&&y"},
			{"regex_with_in_substring", `.msg =~ ".* in .*"`, ".msg", WhenRe, ".* in .*"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := ParseWhen(tc.in)
				require.NoError(t, err)
				assert.Equal(t, tc.wantPath, got.LeftPath)
				assert.Equal(t, tc.wantOp, got.Op)
				assert.Equal(t, tc.wantVal, got.Value)
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		for _, in := range []string{
			"",
			"   ",
			`pod == "x"`,
			`.pod == "trailing" junk`,
			`. == "x"`,
			`.a == "1" && .b == "2"`,
			`.msg.test == "{{ .test }}"`,
			`.a != "{{ .b.c }}"`,
			`.x == {{ .y }}`,
		} {
			_, err := ParseWhen(in)
			assert.Error(t, err, in)
		}
	})
}
