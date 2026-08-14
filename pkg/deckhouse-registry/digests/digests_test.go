// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package digests_test

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/digests"
)

func TestParseFlat(t *testing.T) {
	got, err := digests.Parse([]byte(`{"controller":"sha256:aaa","webhook":"sha256:bbb"}`))
	require.NoError(t, err)

	assert.False(t, got.IsNested())
	assert.Nil(t, got.ByModule)
	assert.Equal(t, map[string]string{"controller": "sha256:aaa", "webhook": "sha256:bbb"}, got.Images)
	assert.Equal(t, 2, got.Count())
}

func TestParseNested(t *testing.T) {
	got, err := digests.Parse([]byte(`{"ingressNginx":{"controller":"sha256:111"},"userAuthn":{"dex":"sha256:222"}}`))
	require.NoError(t, err)

	assert.True(t, got.IsNested())
	assert.Nil(t, got.Images)
	assert.Equal(t, 2, got.Count())
	assert.ElementsMatch(t, []string{"ingressNginx", "userAuthn"}, got.Modules())

	digest, ok := got.Lookup("ingressNginx", "controller")
	assert.True(t, ok)
	assert.Equal(t, "sha256:111", digest)
}

// TestParseEmpty pins the shape of an empty file: flat with an empty map, so
// callers never have to distinguish nil-flat from nil-nested.
func TestParseEmpty(t *testing.T) {
	got, err := digests.Parse([]byte(`{}`))
	require.NoError(t, err)

	assert.False(t, got.IsNested())
	assert.NotNil(t, got.Images)
	assert.Equal(t, 0, got.Count())
}

// TestParseMixed rejects a file that is neither shape rather than guessing.
func TestParseMixed(t *testing.T) {
	_, err := digests.Parse([]byte(`{"controller":"sha256:aaa","ingressNginx":{"x":"sha256:bbb"}}`))
	require.ErrorIs(t, err, digests.ErrMixedShape)
}

func TestParseInvalid(t *testing.T) {
	for _, raw := range []string{``, `not json`, `[]`, `{"a": 1}`, `{"a": null}`} {
		_, err := digests.Parse([]byte(raw))
		assert.Error(t, err, raw)
	}
}

// TestLookupRejectsModuleOnFlat guards the flat/nested distinction: asking for
// a module on a flat file is a miss, not a silent fallthrough.
func TestLookupRejectsModuleOnFlat(t *testing.T) {
	flat, err := digests.Parse([]byte(`{"controller":"sha256:aaa"}`))
	require.NoError(t, err)

	_, ok := flat.Lookup("ingressNginx", "controller")
	assert.False(t, ok)

	_, ok = flat.Lookup("", "controller")
	assert.True(t, ok)
}

// tarOf builds a tar stream with the given name/content pairs, standing in for
// a flattened image.
func tarOf(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))

		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())

	return bytes.NewReader(buf.Bytes())
}

func TestRead(t *testing.T) {
	stream := tarOf(t, map[string]string{
		"deckhouse/modules/images_digests.json": `{"ingressNginx":{"controller":"sha256:111"}}`,
	})

	got, err := digests.Read(stream, "deckhouse/modules/images_digests.json")
	require.NoError(t, err)

	assert.Equal(t, "deckhouse/modules/images_digests.json", got.Source)
	assert.True(t, got.IsNested())
}

// TestReadNormalizesNames covers the tar-name spellings a builder may emit:
// with a "./" or "/" prefix, both of which must match a plain declared path.
func TestReadNormalizesNames(t *testing.T) {
	for _, name := range []string{
		"images_digests.json",
		"./images_digests.json",
		"/images_digests.json",
	} {
		t.Run(name, func(t *testing.T) {
			stream := tarOf(t, map[string]string{name: `{"controller":"sha256:aaa"}`})

			got, err := digests.Read(stream, digests.FileName)
			require.NoError(t, err)
			assert.Equal(t, digests.FileName, got.Source)
		})
	}
}

// TestReadIgnoresOtherPaths guards against reading a same-named file from the
// wrong directory — a module's file must not satisfy a Deckhouse-image read.
func TestReadIgnoresOtherPaths(t *testing.T) {
	stream := tarOf(t, map[string]string{
		"images_digests.json": `{"controller":"sha256:aaa"}`,
	})

	_, err := digests.Read(stream, "deckhouse/modules/images_digests.json")
	require.ErrorIs(t, err, digests.ErrNotFound)
}

func TestReadNotFound(t *testing.T) {
	stream := tarOf(t, map[string]string{"module.yaml": "name: x\n"})

	_, err := digests.Read(stream, digests.FileName)
	require.ErrorIs(t, err, digests.ErrNotFound)
}

func TestReadKeepsRaw(t *testing.T) {
	raw := `{"controller":"sha256:aaa"}`
	stream := tarOf(t, map[string]string{digests.FileName: raw})

	got, err := digests.Read(stream, digests.FileName)
	require.NoError(t, err)
	assert.JSONEq(t, raw, string(got.Raw))
}
