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

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestParseVersionMetadataConversions(t *testing.T) {
	for _, extension := range []string{".yaml", ".yml"} {
		t.Run(extension, func(t *testing.T) {
			img := metadataTar(t, map[string]string{
				"openapi/conversions/v2" + extension: "version: 2\nconversions:\n  - del(.obsolete)\n",
				"openapi/conversions/testcases.yaml": "testcases:\n  - name: remove obsolete setting\n    currentVersion: 1\n    expectedVersion: 2\n",
				"openapi/conversions/notes.yaml":     "not: [valid",
				"openapi/conversions/nested/v3.yaml": "not: [valid",
				"other/v3.yaml":                      "not: [valid",
			})

			meta, err := (&reconciler{}).parseVersionMetadataByImage(context.Background(), img)

			require.NoError(t, err)
			require.NotNil(t, meta.schemas)
			require.Equal(t, []v1alpha1.PackageSettingsConversion{{
				Version: 2,
				Expr:    []string{"del(.obsolete)"},
			}}, meta.schemas.SettingsConversions)
		})
	}
}

func TestParseVersionMetadataRejectsEmptyConversion(t *testing.T) {
	for _, extension := range []string{".yaml", ".yml"} {
		t.Run(extension, func(t *testing.T) {
			img := metadataTar(t, map[string]string{
				"openapi/conversions/v2" + extension: "version: 2\n",
			})

			_, err := (&reconciler{}).parseVersionMetadataByImage(context.Background(), img)

			require.EqualError(t, err, "parse package schemas: settings conversions: conversion version 2: no conversions")
		})
	}
}

func metadataTar(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()

	var img bytes.Buffer
	writer := tar.NewWriter(&img)
	for name, content := range files {
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := writer.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	return &img
}
