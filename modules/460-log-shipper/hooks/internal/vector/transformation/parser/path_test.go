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

func TestParseLabelPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"single", ".foo", []string{"foo"}},
		{"nested", ".a.b.c", []string{"a", "b", "c"}},
		{"double_quoted", `.msg."a-b".level`, []string{"msg", "a-b", "level"}},
		{"single_quoted", `.msg.'a-b'`, []string{"msg", "a-b"}},
		{"escaped_quote", `.k."a\"b"`, []string{"k", `a"b`}},
		{"extra_dots", ".foo..bar", []string{"foo", "bar"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLabelPath(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	for _, in := range []string{"", "   ", "foo", ".", `.foo."bar`} {
		_, err := ParseLabelPath(in)
		assert.Error(t, err, in)
	}
}

func TestSinkKeysFromVRLPaths(t *testing.T) {
	got, err := SinkKeysFromVRLPaths([]string{".pod_labels", ".a.b.c"})
	require.NoError(t, err)
	assert.Equal(t, []string{"pod_labels", "a.b.c"}, got)

	got, err = SinkKeysFromVRLPaths([]string{`.msg."x-y".z`})
	require.NoError(t, err)
	assert.Equal(t, []string{`msg."x-y".z`}, got)

	got, err = SinkKeysFromVRLPaths(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMatchMustachePath(t *testing.T) {
	path, ok := MatchMustachePath(`{{ .foo.bar }}`)
	assert.True(t, ok)
	assert.Equal(t, ".foo.bar", path)

	_, ok = MatchMustachePath(`plain`)
	assert.False(t, ok)
}

func TestMatchMustacheGroup(t *testing.T) {
	group, ok := MatchMustacheGroup(`{{ grp }}`)
	assert.True(t, ok)
	assert.Equal(t, "grp", group)

	_, ok = MatchMustacheGroup(`literal`)
	assert.False(t, ok)
}
