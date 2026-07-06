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

package vcd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func newTestPreparator(client cloudClient) *MetaConfigPreparator {
	p := NewMetaConfigPreparator()

	p.clientProvider = func(_ map[string]json.RawMessage) (cloudClient, error) {
		return client, nil
	}

	return p
}

func TestPreparatorWithCurrentAPI(t *testing.T) {
	preparator := newTestPreparator(testGetCurrentClient())
	result, err := preparator.Prepare(context.TODO(), config.ProviderInput{})

	require.NoError(t, err)
	require.Nil(t, result.ProviderClusterConfig)
}

func TestPreparatorWithLegacyAPI(t *testing.T) {
	preparator := newTestPreparator(testGetLegacyClient())

	result, err := preparator.Prepare(context.TODO(), config.ProviderInput{})
	require.NoError(t, err)
	require.NotNil(t, result.ProviderClusterConfig)
	require.Contains(t, result.ProviderClusterConfig, "legacyMode")
	require.Equal(t, true, result.ProviderClusterConfig["legacyMode"])

	// does not override if legacyMode already set
	legacyModeRaw, _ := json.Marshal(false)
	inputWithLegacy := config.ProviderInput{
		ProviderClusterConfig: map[string]json.RawMessage{"legacyMode": legacyModeRaw},
	}
	result, err = preparator.Prepare(context.TODO(), inputWithLegacy)
	require.NoError(t, err)
	require.Nil(t, result.ProviderClusterConfig)
}

func TestValidateMetaConfig(t *testing.T) {
	const validServer = "https://myserver:8080/api"

	makeInput := func(server, prefix string) config.ProviderInput {
		p, err := json.Marshal(providerConfig{Server: server})
		require.NoError(t, err)
		return config.ProviderInput{
			ClusterPrefix: prefix,
			ProviderClusterConfig: map[string]json.RawMessage{
				"provider": p,
			},
		}
	}

	assertPrefix := func(t *testing.T, prefix string, hasError bool) {
		preparator := newTestPreparator(testGetLegacyClient())
		err := preparator.Validate(context.TODO(), makeInput(validServer, prefix))
		if hasError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	assertPrefix(t, "", true)
	assertPrefix(t, "1abc", false)
	assertPrefix(t, "abc-abc", false)

	preparator := newTestPreparator(testGetLegacyClient())
	err := preparator.Validate(context.TODO(), makeInput("https://myserver:8080/api/", "test"))
	require.Error(t, err)
}
