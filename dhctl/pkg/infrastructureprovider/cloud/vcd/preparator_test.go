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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func newTestPreparator(prepareConfig bool, client cloudClient) *MetaConfigPreparator {
	p := NewMetaConfigPreparator(MetaConfigPreparatorParams{
		PrepareMetaConfig:     prepareConfig,
		ValidateClusterPrefix: true,
	}, log.GetDefaultLogger())

	p.clientProvider = func(_ *config.MetaConfig, _ log.Logger) (cloudClient, error) {
		return client, nil
	}

	return p
}

func TestDisableMetaConfigPreparator(t *testing.T) {
	preparator := newTestPreparator(false, testGetLegacyClient())
	cfg := &config.MetaConfig{}
	err := preparator.Prepare(context.TODO(), cfg)

	require.NoError(t, err)
	require.Nil(t, cfg.ProviderClusterConfig)
}

func TestPreparatorWithCurrentAPI(t *testing.T) {
	preparator := newTestPreparator(false, testGetCurrentClient())
	cfg := &config.MetaConfig{}
	err := preparator.Prepare(context.TODO(), cfg)

	require.NoError(t, err)
	require.Nil(t, cfg.ProviderClusterConfig)
}

func TestPreparatorWithLegacyAPI(t *testing.T) {
	assertLegacyMode := func(t *testing.T, cfg *config.MetaConfig, expect bool) {
		require.NotNil(t, cfg.ProviderClusterConfig)
		require.Contains(t, cfg.ProviderClusterConfig, "legacyMode")

		var res bool
		err := json.Unmarshal(cfg.ProviderClusterConfig["legacyMode"], &res)
		require.NoError(t, err)

		require.Equal(t, res, expect)
	}

	preparator := newTestPreparator(true, testGetLegacyClient())
	cfg := &config.MetaConfig{}
	cfg.ProviderClusterConfig = make(map[string]json.RawMessage)
	err := preparator.Prepare(context.TODO(), cfg)

	require.NoError(t, err)
	assertLegacyMode(t, cfg, true)

	// does not prepare if legacy mode is set

	cfgWithLegacy := &config.MetaConfig{}
	cfgWithLegacy.ProviderClusterConfig = make(map[string]json.RawMessage)
	legacyMode, err := json.Marshal(false)
	require.NoError(t, err)
	cfgWithLegacy.ProviderClusterConfig["legacyMode"] = legacyMode

	err = preparator.Prepare(context.TODO(), cfgWithLegacy)
	require.NoError(t, err)
	assertLegacyMode(t, cfgWithLegacy, false)
}

func TestValidateMetaConfig(t *testing.T) {
	const validServer = "https://myserver:8080/api"

	setServer := func(t *testing.T, server string, cfg *config.MetaConfig) {
		p, err := json.Marshal(providerConfig{
			Server: server,
		})
		require.NoError(t, err)

		cfg.ProviderClusterConfig = map[string]json.RawMessage{
			"provider": p,
		}
	}

	assertPrefix := func(t *testing.T, prefix string, hasError bool) {
		preparator := newTestPreparator(true, testGetLegacyClient())

		cfg := &config.MetaConfig{}

		setServer(t, validServer, cfg)

		cfg.ClusterPrefix = prefix
		err := preparator.Validate(context.TODO(), cfg)

		if hasError {
			require.Error(t, err)
			return
		}

		require.NoError(t, err)
	}

	assertPrefix(t, "", true)
	assertPrefix(t, "1abc", false)
	assertPrefix(t, "abc-abc", false)

	preparator := newTestPreparator(false, testGetLegacyClient())
	preparator.params.ValidateClusterPrefix = false
	cfg := &config.MetaConfig{}

	cfg.ClusterPrefix = ""
	setServer(t, validServer, cfg)
	err := preparator.Validate(context.TODO(), cfg)
	require.NoError(t, err)

	// invalid server
	cfgInvalid := &config.MetaConfig{}

	cfgInvalid.ClusterPrefix = "test"
	setServer(t, "https://myserver:8080/api/", cfg)
	err = preparator.Validate(context.TODO(), cfg)
	require.Error(t, err)
}
