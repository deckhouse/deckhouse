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

package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestLoadSchemasConversionFiles(t *testing.T) {
	for name, load := range map[string]func(string) (loader.RawSchemas, error){
		"embedded": loader.LoadEmbeddedSchemas,
		"global":   loader.LoadGlobalSchemas,
	} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name    string
				file    string
				content string
				wantErr string
			}{
				{
					name:    "yaml",
					file:    "v2.yaml",
					content: "version: 2\nconversions:\n  - .new = .old | del(.old)\n",
				},
				{
					name:    "yml",
					file:    "v2.yml",
					content: "version: 2\nconversions:\n  - .new = .old | del(.old)\n",
				},
				{
					name:    "missing expressions",
					file:    "v2.yaml",
					content: "version: 2\n",
					wantErr: "settings conversions: conversion version 2: no conversions",
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					conversionsDir := filepath.Join(dir, "openapi", "conversions")
					require.NoError(t, os.MkdirAll(conversionsDir, 0755))
					for file, content := range map[string]string{
						tc.file:          tc.content,
						"testcases.yaml": "testcases:\n  - name: rename setting\n    currentVersion: 1\n    expectedVersion: 2\n",
						"notes.yaml":     "description: conversion examples\n",
					} {
						require.NoError(t, os.WriteFile(filepath.Join(conversionsDir, file), []byte(content), 0644))
					}

					raw, err := load(dir)
					require.NoError(t, err)
					require.Equal(t, [][]byte{[]byte(tc.content)}, raw.Conversions)

					schemas, err := v1alpha1.ParsePackageSchemas(raw.Settings, raw.Values, raw.Conversions)
					if tc.wantErr != "" {
						require.EqualError(t, err, tc.wantErr)
						return
					}
					require.NoError(t, err)
					require.Len(t, schemas.SettingsConversions, 1)
					require.Equal(t, 2, schemas.SettingsConversions[0].Version)
					require.Equal(t, []string{".new = .old | del(.old)"}, schemas.SettingsConversions[0].Expr)
				})
			}
		})
	}
}
