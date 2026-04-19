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

package yandex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestValidateClusterPrefix(t *testing.T) {
	getMetaConfig := func(clusterPrefix string) *config.MetaConfig {
		cfg := &config.MetaConfig{}

		cfg.ClusterPrefix = clusterPrefix
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		fillTestProviderClusterConfig(cfg, master, nil)

		return cfg
	}
	assertClusterPrefix := func(t *testing.T, clusterPrefix string, hasError bool) {
		assertValidation(t, true, getMetaConfig(clusterPrefix), hasError)
	}

	assertClusterPrefix(t, "", true)
	assertClusterPrefix(t, "1abbbs", true)
	assertClusterPrefix(t, strings.Repeat("a", 100), true)
	assertClusterPrefix(t, "abc-abc", false)

	assertValidation(t, false, getMetaConfig(""), false)
}

func TestValidateMasterNodeGroupSpec(t *testing.T) {
	assertMasterNodeGroup := func(t *testing.T, replicas int, externalIPS []string, hasError bool) {
		cfg := getTestCfgForMaster(t, replicas, externalIPS)
		assertValidation(t, true, cfg, hasError)
	}

	assertMasterNodeGroup(t, 2, []string{"1.1.1.1"}, true)
	assertMasterNodeGroup(t, 1, []string{}, false)
	assertMasterNodeGroup(t, 1, []string{"1.1.1.1"}, false)
	assertMasterNodeGroup(t, 2, []string{"1.1.1.1", "2.2.2.2"}, false)
}

func TestValidateNodeGroupsSpec(t *testing.T) {
	assertNodeGroups := func(t *testing.T, replicas int, externalIPS []string, hasError bool) {
		cfg := &config.MetaConfig{}

		cfg.ClusterPrefix = "valid-prefix"
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		nodeGroups := getTestNodeGroupsSpec(t, replicas, externalIPS)
		fillTestProviderClusterConfig(cfg, master, nodeGroups)

		assertValidation(t, true, cfg, hasError)
	}

	assertNodeGroups(t, 2, []string{"1.1.1.1"}, true)
	assertNodeGroups(t, 1, []string{}, false)
	assertNodeGroups(t, 1, []string{"1.1.1.1"}, false)
	assertNodeGroups(t, 2, []string{"1.1.1.1", "2.2.2.2"}, false)
}

func TestWithNATInstanceLayoutSpec(t *testing.T) {
	getMetaConfig := func(t *testing.T, settings string, nodeGroups json.RawMessage) *config.MetaConfig {
		cfg := &config.MetaConfig{}

		cfg.ClusterPrefix = "valid-prefix"
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		fillTestProviderClusterConfig(cfg, master, nodeGroups)
		fillTestWithNatInstanceLayout(t, cfg, settings)

		return cfg
	}

	assertWithNATInstance := func(t *testing.T, settings string, hasError bool, nodeGroups json.RawMessage) {
		cfg := getMetaConfig(t, settings, nodeGroups)
		assertValidation(t, true, cfg, hasError)
	}

	// no settings
	assertWithNATInstance(t, "", true, nil)
	// empty settings
	assertWithNATInstance(t, `{}`, true, nil)
	// no required values
	assertWithNATInstance(t, `{"exporterAPIKey": "not security key"}`, true, nil)
	// both is correct. tofu select in our logic order
	assertWithNATInstance(t, `{"internalSubnetID": "id", "internalSubnetCIDR": "127.0.0.1/24"}`, false, nil)
	// only id
	assertWithNATInstance(t, `{"internalSubnetID": "id"}`, false, nil)
	// only cidr
	assertWithNATInstance(t, `{"internalSubnetCIDR": "127.0.0.1/24"}`, false, nil)
	// all in
	assertWithNATInstance(t,
		`{"internalSubnetCIDR": "127.0.0.1/24"}`,
		false,
		getTestNodeGroupsSpec(t, 1, []string{"1.1.1.1"}),
	)

	assertSkipValidationWithNATInstance := func(t *testing.T, settings string, nodeGroups json.RawMessage) {
		cfg := getMetaConfig(t, settings, nodeGroups)
		preparator := NewMetaConfigPreparator(true)
		require.False(t, preparator.validateWithNATLayout)

		err := preparator.Validate(context.TODO(), cfg)
		require.NoError(t, err)
	}

	// skip with-nat layout validation
	// no settings
	assertSkipValidationWithNATInstance(t, "", nil)
	// empty settings
	assertSkipValidationWithNATInstance(t, `{}`, nil)
	// no required values
	assertSkipValidationWithNATInstance(t, `{"exporterAPIKey": "not security key"}`, nil)
	// all in
	assertSkipValidationWithNATInstance(t,
		`{"internalSubnetCIDR": "127.0.0.1/24"}`,
		getTestNodeGroupsSpec(t, 1, []string{"1.1.1.1"}),
	)
}

func TestSetIncorrectLogDoesNotPanic(t *testing.T) {
	cfg := getTestCfgForMaster(t, 1, []string{"1.1.1.1"})

	do := func() {
		preparator := NewMetaConfigPreparator(true)

		preparator.WithLogger(nil)

		_ = preparator.Validate(context.TODO(), cfg)
	}

	require.NotPanics(t, do)
}

func getTestCfgForMaster(t *testing.T, replicas int, externalIPS []string) *config.MetaConfig {
	cfg := &config.MetaConfig{}

	cfg.ClusterPrefix = "valid-prefix"
	master := getTestMasterNodeGroupSpec(t, replicas, externalIPS)
	fillTestProviderClusterConfig(cfg, master, nil)

	return cfg
}

func getTestMasterNodeGroupSpec(t *testing.T, replicas int, externalIPs []string) json.RawMessage {
	spec := masterNodeGroupSpec{
		Replicas: replicas,
		InstanceClass: instanceClass{
			ExternalIPAddresses: externalIPs,
		},
	}

	b, err := json.Marshal(spec)
	require.NoError(t, err)

	return b
}

func getTestNodeGroupsSpec(t *testing.T, replicas int, externalIPs []string) json.RawMessage {
	spec := []nodeGroupSpec{
		{
			Name:     "test",
			Replicas: replicas,
			InstanceClass: instanceClass{
				ExternalIPAddresses: externalIPs,
			},
		},
	}

	b, err := json.Marshal(spec)
	require.NoError(t, err)

	return b
}

func fillTestProviderClusterConfig(cfg *config.MetaConfig, master json.RawMessage, nodeGroups json.RawMessage) {
	cfg.ProviderClusterConfig = make(map[string]json.RawMessage)

	cfg.ProviderClusterConfig["masterNodeGroup"] = master

	if len(nodeGroups) > 0 {
		cfg.ProviderClusterConfig["nodeGroups"] = nodeGroups
	}
}

func fillTestWithNatInstanceLayout(t *testing.T, cfg *config.MetaConfig, settings string) {
	t.Helper()

	require.NotEmpty(t, cfg.ProviderClusterConfig)

	cfg.Layout = "with-nat-instance"

	if settings != "" {
		cfg.ProviderClusterConfig["withNATInstance"] = json.RawMessage([]byte(settings))
	}
}

func assertValidation(t *testing.T, validatePrefix bool, cfg *config.MetaConfig, hasError bool) {
	preparator := NewMetaConfigPreparator(validatePrefix).EnableValidateWithNATLayout()

	require.True(t, preparator.validateWithNATLayout)

	err := preparator.Validate(context.TODO(), cfg)
	if hasError {
		require.Error(t, err)
		return
	}

	require.NoError(t, err)
}
