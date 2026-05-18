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
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
)

func TestPreparatorValidKubeconfig(t *testing.T) {
	kubeconfig := base64.StdEncoding.EncodeToString([]byte(testKubeconfig()))
	raw, err := json.Marshal(DVPProviderSpec{KubeconfigDataBase64: kubeconfig})
	require.NoError(t, err)

	input := config.ProviderInput{
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"provider": raw,
		},
		CloudProviderVars: parseTestResourcesYAML(t),
	}

	// bootstrap + ValidateKubeAPI=false — parses kubeconfig but skips the API call.
	prep := NewPreparator("bootstrap", PreparatorOptions{ValidateKubeAPI: false})
	err = prep.Validate(context.Background(), input)
	require.NoError(t, err)
}

func TestPreparatorMissingKubeconfigBase64(t *testing.T) {
	raw, err := json.Marshal(DVPProviderSpec{KubeconfigDataBase64: ""})
	require.NoError(t, err)

	input := config.ProviderInput{
		ProviderName: "dvp",
		ProviderClusterConfig: map[string]json.RawMessage{
			"provider": raw,
		},
		CloudProviderVars: parseTestResourcesYAML(t),
	}

	prep := NewPreparator("bootstrap", PreparatorOptions{ValidateKubeAPI: false})
	err = prep.Validate(context.Background(), input)
	require.ErrorContains(t, err, "kubeconfigDataBase64 must be set")
}

func TestPreparatorMissingSecret(t *testing.T) {
	input := config.ProviderInput{
		ProviderName:      "dvp",
		CloudProviderVars: &providerdata.CloudProviderVars{},
	}

	prep := NewPreparator("", PreparatorOptions{})
	err := prep.Validate(context.Background(), input)
	require.ErrorContains(t, err, "no credential Secret found")
}

// parseTestResourcesYAML parses testResourcesYAML into CloudProviderVars
// the same way the production code does via providerdata.CloudProviderVarsFromInput.
func parseTestResourcesYAML(t *testing.T) *providerdata.CloudProviderVars {
	t.Helper()
	cv, err := providerdata.CloudProviderVarsFromInput(context.Background(), providerdata.PrepareInput{
		ResourcesYAML: testResourcesYAML(),
	})
	require.NoError(t, err)
	return cv
}

func testResourcesYAML() string {
	return `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      name: worker
---
apiVersion: deckhouse.io/v1
kind: DVPInstanceClass
metadata:
  name: worker
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: dvp-credentials
type: cloud-provider.deckhouse.io/credentials
stringData:
  token: test
`
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
