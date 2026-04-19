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

package dvp

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestKubeconfigDataBase64HappyPath(t *testing.T) {
	preparator := NewMetaConfigPreparator()

	kubeconfig := base64.StdEncoding.EncodeToString([]byte(testKubeconfig()))
	metaCfg := metaConfigWithProvider(t, DVPProviderSpec{KubeconfigDataBase64: kubeconfig})

	_, err := preparator.KubeconfigDataBase64(metaCfg)
	require.NoError(t, err)
}

func metaConfigWithProvider(t *testing.T, spec DVPProviderSpec) *config.MetaConfig {
	raw, err := json.Marshal(spec)
	require.NoError(t, err)

	return &config.MetaConfig{
		ProviderClusterConfig: map[string]json.RawMessage{
			"provider": raw,
		},
	}
}

func testKubeconfig() string {
	return `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://flat.com
    insecure-skip-tls-verify: true
contexts:
- name: c
  context:
    cluster: c
    user: u
users:
- name: u
  user:
    token: bobobbob==
current-context: c
`
}
