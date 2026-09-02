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

package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderSystemScriptsWithCloudProviderSecret(t *testing.T) {
	tests := []struct {
		name            string
		secrets         map[string]cloudProviderStepsSecret
		expectedScripts map[string]string
	}{
		{
			name:    "uses legacy provider steps when Secret is absent",
			secrets: map[string]cloudProviderStepsSecret{},
			expectedScripts: map[string]string{
				"001_common.sh": "common",
				"002_legacy.sh": "legacy",
			},
		},
		{
			name: "uses Secret instead of legacy provider steps",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-steps": {
					provider: "aws",
					data: map[string][]byte{
						"003_external.sh.tpl": []byte("external"),
					},
				},
			},
			expectedScripts: map[string]string{
				"001_common.sh":   "common",
				"003_external.sh": "external",
			},
		},
		{
			name: "empty Secret disables legacy provider steps",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-steps": {
					provider: "aws",
					data:     map[string][]byte{},
				},
			},
			expectedScripts: map[string]string{
				"001_common.sh": "common",
			},
		},
		{
			name: "Secret step overrides common step",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-steps": {
					provider: "aws",
					data: map[string][]byte{
						"001_common.sh.tpl": []byte("external replacement"),
					},
				},
			},
			expectedScripts: map[string]string{
				"001_common.sh": "external replacement",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &StepsStorage{
				// "all:" represents common Deckhouse steps.
				// "all:aws" represents the old combined common + provider cache.
				systemScripts: map[string]map[string][]byte{
					"all:": {
						"001_common.sh.tpl": []byte("common"),
					},
					"all:aws": {
						"001_common.sh.tpl": []byte("common"),
						"002_legacy.sh.tpl": []byte("legacy"),
					},
				},
				cloudProviderStepSecrets: tt.secrets,
			}

			steps, err := storage.renderSystemScripts(
				"all",
				"aws",
				map[string]interface{}{},
			)

			require.NoError(t, err)
			require.Equal(t, tt.expectedScripts, steps)

			// Combining Secret steps must not modify the cached common templates.
			require.Equal(
				t,
				"common",
				string(storage.systemScripts["all:"]["001_common.sh.tpl"]),
			)
		})
	}
}
