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

func testClientProvider(client cloudClient) clientProvider {
	return func(_ map[string]json.RawMessage) (cloudClient, error) {
		return client, nil
	}
}

func TestEnsureLegacyModeCurrentAPI(t *testing.T) {
	// nil PCC must not panic: the map is created on demand.
	m := &config.MetaConfig{}
	require.NoError(t, ensureLegacyMode(t.Context(), m, testClientProvider(testGetCurrentClient())))
	require.JSONEq(t, "false", string(m.ProviderClusterConfig["legacyMode"]))
}

func TestEnsureLegacyModeLegacyAPI(t *testing.T) {
	m := &config.MetaConfig{ProviderClusterConfig: map[string]json.RawMessage{}}
	require.NoError(t, ensureLegacyMode(context.TODO(), m, testClientProvider(testGetLegacyClient())))
	require.JSONEq(t, "true", string(m.ProviderClusterConfig["legacyMode"]))

	// does not override a user-set legacyMode and makes no client call
	legacyModeRaw, _ := json.Marshal(false)
	m = &config.MetaConfig{ProviderClusterConfig: map[string]json.RawMessage{"legacyMode": legacyModeRaw}}
	require.NoError(t, ensureLegacyMode(t.Context(), m, func(map[string]json.RawMessage) (cloudClient, error) {
		t.Fatal("client must not be built when legacyMode is already set")
		return nil, nil
	}))
	require.JSONEq(t, "false", string(m.ProviderClusterConfig["legacyMode"]))
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
		err := ValidateMetaConfig(t.Context(), makeInput(validServer, prefix))
		if hasError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	assertPrefix(t, "", true)
	assertPrefix(t, "1abc", false)
	assertPrefix(t, "abc-abc", false)

	err := ValidateMetaConfig(t.Context(), makeInput("https://myserver:8080/api/", "test"))
	require.Error(t, err)
}
