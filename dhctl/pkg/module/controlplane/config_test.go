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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestSignatureMode(t *testing.T) {
	type test struct {
		name    string
		edition string
		cfg     *config.MetaConfig
		isErr   bool
		res     string
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

	tests := []test{
		{
			name:    "not cse without mc",
			edition: "fe",
			cfg:     &config.MetaConfig{},
			res:     NoSignatureMode,
			isErr:   false,
		},

		{
			name:    "not cse with mc settings",
			edition: "fe",
			cfg: metaConfig(map[string]any{
				"apiserver": map[string]any{
					"signature": "Migrate",
				},
			}),
			res:   NoSignatureMode,
			isErr: false,
		},

		{
			name:    "cse without mc",
			edition: "cse",
			cfg:     &config.MetaConfig{},
			res:     defaultSignatureMode,
			isErr:   false,
		},

		{
			name:    "cse with mc no settings",
			edition: "cse",
			cfg:     metaConfig(nil),
			res:     defaultSignatureMode,
			isErr:   false,
		},

		{
			name:    "cse with mc no apiserver",
			edition: "cse",
			cfg: metaConfig(map[string]any{
				"enabledFeatureGates": []string{
					"MyGate",
				},
			}),
			res:   defaultSignatureMode,
			isErr: false,
		},

		{
			name:    "cse with mc with apiserver no mode",
			edition: "cse",
			cfg: metaConfig(map[string]any{
				"apiserver": map[string]any{
					"encryptionEnabled": true,
				},
			}),
			res:   defaultSignatureMode,
			isErr: false,
		},

		{
			name:    "cse with mc with apiserver with default mode",
			edition: "cse",
			cfg: metaConfig(map[string]any{
				"apiserver": map[string]any{
					"encryptionEnabled": true,
					"signature":         "Migrate",
				},
			}),
			res:   defaultSignatureMode,
			isErr: false,
		},

		{
			name:    "cse with mc with apiserver with not default mode",
			edition: "cse",
			cfg: metaConfig(map[string]any{
				"apiserver": map[string]any{
					"signature": "Enforce",
				},
			}),
			res:   "Enforce",
			isErr: false,
		},

		{
			name:    "cse with mc with apiserver incorrect signature type",
			edition: "cse",
			cfg: metaConfig(map[string]any{
				"apiserver": map[string]any{
					"signature": 42,
				},
			}),
			res:   "",
			isErr: true,
		},

		{
			name:    "cse with multiple mc's with apiserver with not default mode",
			edition: "cse",
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
			res:   "Rollback",
			isErr: false,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			assertError := require.NoError
			if tst.isErr {
				assertError = require.Error
			}

			logger := log.NewInMemoryLoggerWithParent(log.NewDummyLogger(true))

			s := NewSettingsExtractor(tst.cfg, tst.edition, log.SimpleLoggerProvider(logger))

			res, err := s.SignatureMode()
			assertError(t, err)

			require.Equal(t, tst.res, res, "should correct signature mode")
		})
	}
}
