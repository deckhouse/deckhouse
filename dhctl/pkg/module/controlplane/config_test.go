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

package controlplane

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestSignatureMode(t *testing.T) {
	type test struct {
		name        string
		edition     string
		cfg         *config.MetaConfig
		isErr       bool
		res         string
		schemaStore SchemaStore
	}

	if _, err := os.Stat("/deckhouse/ee/cse/modules/040-control-plane-manager/openapi/config-values.yaml"); err != nil {
		t.Skip("TestSignatureMode available only local run now")
	}

	notEESchemaStore := func() SchemaStore {
		return newTestSchemaStore("/deckhouse/modules/040-control-plane-manager/openapi/config-values.yaml")
	}

	eeSchemaStore := func() SchemaStore {
		return newTestSchemaStore("/deckhouse/ee/modules/040-control-plane-manager/openapi/config-values.yaml")
	}

	cseSchemaStore := func() SchemaStore {
		return newTestSchemaStore("/deckhouse/ee/cse/modules/040-control-plane-manager/openapi/config-values.yaml")
	}

	metaConfig := func(set map[string]any) *config.MetaConfig {
		return &config.MetaConfig{
			ModuleConfigs: []*config.ModuleConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-manager",
					},

					Spec: config.ModuleConfigSpec{
						Settings: set,
					},
				},
			},
		}
	}

	tests := make([]test, 0)

	// not ee tests
	for _, ed := range []string{"ce", "be", "se", "se-plus"} {
		suit := []test{
			{
				name:        fmt.Sprintf("not ee (%s) without mc", ed),
				edition:     ed,
				cfg:         &config.MetaConfig{},
				res:         NoSignatureMode,
				isErr:       false,
				schemaStore: notEESchemaStore(),
			},

			{
				name:    fmt.Sprintf("not ee (%s) with mc settings", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"apiserver": map[string]any{
						"signature": "Migrate",
					},
				}),
				res:         NoSignatureMode,
				isErr:       false,
				schemaStore: notEESchemaStore(),
			},
		}

		tests = append(tests, suit...)
	}

	createEESuits := func(ed string, defaultMode string, schemaStore SchemaStore) []test {
		return []test{
			{
				name:        fmt.Sprintf("ee (%s) without mc", ed),
				edition:     ed,
				cfg:         &config.MetaConfig{},
				res:         defaultMode,
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:        fmt.Sprintf("ee (%s) with mc no settings", ed),
				edition:     ed,
				cfg:         metaConfig(nil),
				res:         defaultMode,
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with mc no apiserver", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"enabledFeatureGates": []string{
						"MyGate",
					},
				}),
				res:         defaultMode,
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with mc with apiserver no mode", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"apiserver": map[string]any{
						"encryptionEnabled": true,
					},
				}),
				res:         defaultMode,
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with mc with apiserver with migrate", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"apiserver": map[string]any{
						"encryptionEnabled": true,
						"signature":         "Migrate",
					},
				}),
				res:         "Migrate",
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with mc with apiserver with not default mode", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"apiserver": map[string]any{
						"signature": "Enforce",
					},
				}),
				res:         "Enforce",
				isErr:       false,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with mc with apiserver incorrect signature type", ed),
				edition: ed,
				cfg: metaConfig(map[string]any{
					"apiserver": map[string]any{
						"signature": 42,
					},
				}),
				res:         "",
				isErr:       true,
				schemaStore: schemaStore,
			},

			{
				name:    fmt.Sprintf("ee (%s) with multiple mc's with apiserver with not default mode", ed),
				edition: ed,
				cfg: func() *config.MetaConfig {
					m := metaConfig(map[string]any{
						"apiserver": map[string]any{
							"signature": "Rollback",
						},
					})

					m.ModuleConfigs = append(m.ModuleConfigs, &config.ModuleConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name: "deckhouse",
						},
						Spec: config.ModuleConfigSpec{
							Settings: map[string]any{
								"releaseChannel": "LTS",
							},
						},
					})

					return m
				}(),
				res:         "Rollback",
				isErr:       false,
				schemaStore: schemaStore,
			},
		}
	}

	// ee tests
	for _, ed := range []string{"ee", "fe"} {
		tests = append(tests, createEESuits(ed, "Rollback", eeSchemaStore())...)
	}

	tests = append(tests, createEESuits("cse", "Migrate", cseSchemaStore())...)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			assertError := require.NoError
			if tst.isErr {
				assertError = require.Error
			}

			logger := log.NewInMemoryLoggerWithParent(log.NewDummyLogger(true))

			s := NewSettingsExtractor(tst.cfg, tst.schemaStore, tst.edition, log.SimpleLoggerProvider(logger))

			res, err := s.SignatureMode()
			assertError(t, err)

			require.Equal(t, tst.res, res, "should correct signature mode")
		})
	}
}

type testSchemaStore struct {
	path string
}

func newTestSchemaStore(path string) *testSchemaStore {
	return &testSchemaStore{
		path: path,
	}
}

func (s *testSchemaStore) GetModuleConfigSchema(string) (*spec.Schema, error) {
	content, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	schema := new(spec.Schema)
	if err := yaml.Unmarshal(content, schema); err != nil {
		return nil, err
	}

	return schema, nil
}
