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

package parserlabels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWhen(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			in            string
			wantPath      string
			wantOp        WhenOp
			wantVal       string
			wantRightPath []string
		}{
			{"eq_unquoted", `.pod == foo`, ".pod", WhenEQ, "foo", nil},
			{"eq_double", `.pod == "foo"`, ".pod", WhenEQ, "foo", nil},
			{"ne_single", `.pod  !=  'bar'`, ".pod", WhenNE, "bar", nil},
			{"=~", `.msg =~ ".*"`, ".msg", WhenRe, ".*", nil},
			{"=~_unquoted", `.msg =~ .*`, ".msg", WhenRe, ".*", nil},
			{"!=~", `.msg !=~ "[0-9]+"`, ".msg", WhenNRe, "[0-9]+", nil},
			{"nested", `.a.b !=~ 'x'`, ".a.b", WhenNRe, "x", nil},
			{"trim", ` .nested.path == "v" `, ".nested.path", WhenEQ, "v", nil},
			{"escaped_in_value", ".k == \"a\\\"b\"", ".k", WhenEQ, `a"b`, nil},
			{"quoted_value_with_eq_inside", `.path =~ "x==y"`, ".path", WhenRe, "x==y", nil},
			{"value_with_double_amp", `.a == "x&&y"`, ".a", WhenEQ, "x&&y", nil},
			{"eq_mustache_right", `.msg.test == "{{ .test }}"`, ".msg.test", WhenEQ, "{{ .test }}", []string{"test"}},
			{"ne_mustache_right", `.a != "{{ .b.c }}"`, ".a", WhenNE, "{{ .b.c }}", []string{"b", "c"}},
			{"regex_with_in_substring", `.msg =~ ".* in .*"`, ".msg", WhenRe, ".* in .*", nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := ParseWhen(tc.in)
				require.NoError(t, err)
				assert.Equal(t, tc.wantPath, got.LeftPath)
				assert.Equal(t, tc.wantOp, got.Op)
				assert.Equal(t, tc.wantVal, got.Value)
				assert.Equal(t, tc.wantRightPath, got.RightPathSegs)
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
		} {
			_, err := ParseWhen(in)
			assert.Error(t, err, in)
		}
	})
}
