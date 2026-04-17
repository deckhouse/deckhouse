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

// tarEntry describes one regular file inside a synthetic tar archive.
type tarEntry struct {
	name    string
	content []byte
}

// buildTar packs entries into a tar archive in the order given.
func buildTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(e.content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func newMetadataReader() *metadataReader {
	return &metadataReader{
		definitionReader:     bytes.NewBuffer(nil),
		versionReader:        bytes.NewBuffer(nil),
		changelogReader:      bytes.NewBuffer(nil),
		valuesSchemaReader:   bytes.NewBuffer(nil),
		settingsSchemaReader: bytes.NewBuffer(nil),
	}
}

// TestUntarMetadata_PrefersSettingsYaml verifies that openapi/settings.yaml is
// read into the settings buffer when present.
func TestUntarMetadata_PrefersSettingsYaml(t *testing.T) {
	want := []byte("source: settings")
	archive := buildTar(t, []tarEntry{
		{name: settingsSchemaFile, content: want},
	})

	mr := newMetadataReader()
	require.NoError(t, mr.untarMetadata(bytes.NewReader(archive)))
	require.Equal(t, want, mr.settingsSchemaReader.Bytes())
	require.True(t, mr.settingsSchemaCanonical)
}

// TestUntarMetadata_LegacyConfigValuesFallback verifies that an archive containing
// only the legacy openapi/config-values.yaml is still ingested into the settings buffer.
func TestUntarMetadata_LegacyConfigValuesFallback(t *testing.T) {
	want := []byte("source: legacy")
	archive := buildTar(t, []tarEntry{
		{name: legacySettingsSchemaFile, content: want},
	})

	mr := newMetadataReader()
	require.NoError(t, mr.untarMetadata(bytes.NewReader(archive)))
	require.Equal(t, want, mr.settingsSchemaReader.Bytes())
	require.False(t, mr.settingsSchemaCanonical)
}

// TestUntarMetadata_SettingsWinsWhenLegacyFirst verifies that if both files appear
// in the archive with the legacy name first, the canonical settings.yaml replaces it.
func TestUntarMetadata_SettingsWinsWhenLegacyFirst(t *testing.T) {
	settingsContent := []byte("source: settings")
	legacyContent := []byte("source: legacy")
	archive := buildTar(t, []tarEntry{
		{name: legacySettingsSchemaFile, content: legacyContent},
		{name: settingsSchemaFile, content: settingsContent},
	})

	mr := newMetadataReader()
	require.NoError(t, mr.untarMetadata(bytes.NewReader(archive)))
	require.Equal(t, settingsContent, mr.settingsSchemaReader.Bytes())
	require.True(t, mr.settingsSchemaCanonical)
}

// TestUntarMetadata_SettingsWinsWhenSettingsFirst verifies that if settings.yaml
// is read first and the legacy file appears later, the legacy entry is ignored.
func TestUntarMetadata_SettingsWinsWhenSettingsFirst(t *testing.T) {
	settingsContent := []byte("source: settings")
	legacyContent := []byte("source: legacy")
	archive := buildTar(t, []tarEntry{
		{name: settingsSchemaFile, content: settingsContent},
		{name: legacySettingsSchemaFile, content: legacyContent},
	})

	mr := newMetadataReader()
	require.NoError(t, mr.untarMetadata(bytes.NewReader(archive)))
	require.Equal(t, settingsContent, mr.settingsSchemaReader.Bytes())
	require.True(t, mr.settingsSchemaCanonical)
}
