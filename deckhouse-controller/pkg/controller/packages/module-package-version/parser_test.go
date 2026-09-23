/*
Copyright 2026 Flant JSC

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

package modulepackageversion

import (
	"archive/tar"
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// orderedTar builds a tar stream with the entries in the given order, so tests
// can pin behaviour that depends on the position of a file in the archive.
func orderedTar(t *testing.T, entries [][2]string) *bytes.Buffer {
	t.Helper()

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, entry := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: entry[0], Mode: 0o644, Size: int64(len(entry[1]))}))
		_, err := tw.Write([]byte(entry[1]))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	return buf
}

func TestParseSchemasPrecedenceOverTarOrder(t *testing.T) {
	r := &reconciler{logger: log.NewNop()}

	// config-values.yaml sits before settings.yaml, with every other metadata
	// file already captured in between: the walk must not stop early and the
	// preferred name must still win.
	img := orderedTar(t, [][2]string{
		{"version.json", `{"version": "1.0.0"}`},
		{"package.yaml", "name: test-module\n"},
		{"changelog.yaml", "features:\n- f\n"},
		{"openapi/config-values.yaml", "type: object\nproperties:\n  legacyOption:\n    type: string\n"},
		{"openapi/values.yaml", "type: object\n"},
		{"openapi/settings.yaml", "type: object\nproperties:\n  preferredOption:\n    type: string\n"},
	})

	meta, err := r.parseVersionMetadataByImage(context.Background(), img)
	require.NoError(t, err)

	require.NotNil(t, meta.schemas)
	require.NotNil(t, meta.schemas.SettingsSchema)
	assert.Contains(t, meta.schemas.SettingsSchema.OpenAPIV3Schema.Properties, "preferredOption",
		"settings.yaml must win over config-values.yaml regardless of tar order")
}
