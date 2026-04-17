// Copyright 2025 Flant JSC
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

package applicationpackageversion

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUntarMetadata_LegacyConfigValuesFallback verifies that an archive containing
// the legacy openapi/config-values.yaml is still ingested into the settings buffer.
func TestUntarMetadata_LegacyConfigValuesFallback(t *testing.T) {
	want := []byte("source: legacy")

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: legacySettingsSchemaFile,
		Mode: 0o644,
		Size: int64(len(want)),
	}))
	_, err := tw.Write(want)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	mr := &metadataReader{
		definitionReader:     bytes.NewBuffer(nil),
		versionReader:        bytes.NewBuffer(nil),
		changelogReader:      bytes.NewBuffer(nil),
		valuesSchemaReader:   bytes.NewBuffer(nil),
		settingsSchemaReader: bytes.NewBuffer(nil),
	}
	require.NoError(t, mr.untarMetadata(bytes.NewReader(buf.Bytes())))
	require.Equal(t, want, mr.settingsSchemaReader.Bytes())
}
