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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestValidateClusterPrefix(t *testing.T) {
	getInput := func(clusterPrefix string) config.ProviderInput {
		input := config.ProviderInput{ClusterPrefix: clusterPrefix}
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		fillTestProviderClusterConfig(&input, master, nil)
		return input
	}
	assertClusterPrefix := func(t *testing.T, clusterPrefix string, hasError bool) {
		assertValidation(t, true, getInput(clusterPrefix), hasError)
	}

	assertClusterPrefix(t, "", true)
	assertClusterPrefix(t, "1abbbs", true)
	assertClusterPrefix(t, strings.Repeat("a", 100), true)
	assertClusterPrefix(t, "abc-abc", false)
}

func TestValidateMasterNodeGroupSpec(t *testing.T) {
	assertMasterNodeGroup := func(t *testing.T, replicas int, externalIPS []string, hasError bool) {
		input := getTestInputForMaster(t, replicas, externalIPS)
		assertValidation(t, true, input, hasError)
	}

	assertMasterNodeGroup(t, 2, []string{"1.1.1.1"}, true)
	assertMasterNodeGroup(t, 1, []string{}, false)
	assertMasterNodeGroup(t, 1, []string{"1.1.1.1"}, false)
	assertMasterNodeGroup(t, 2, []string{"1.1.1.1", "2.2.2.2"}, false)
}

func TestValidateNodeGroupsSpec(t *testing.T) {
	assertNodeGroups := func(t *testing.T, replicas int, externalIPS []string, hasError bool) {
		input := config.ProviderInput{ClusterPrefix: "valid-prefix"}
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		nodeGroups := getTestNodeGroupsSpec(t, replicas, externalIPS)
		fillTestProviderClusterConfig(&input, master, nodeGroups)
		assertValidation(t, true, input, hasError)
	}

	assertNodeGroups(t, 2, []string{"1.1.1.1"}, true)
	assertNodeGroups(t, 1, []string{}, false)
	assertNodeGroups(t, 1, []string{"1.1.1.1"}, false)
	assertNodeGroups(t, 2, []string{"1.1.1.1", "2.2.2.2"}, false)
}

func TestWithNATInstanceLayoutSpec(t *testing.T) {
	getInput := func(t *testing.T, settings string, nodeGroups json.RawMessage) config.ProviderInput {
		input := config.ProviderInput{ClusterPrefix: "valid-prefix"}
		master := getTestMasterNodeGroupSpec(t, 1, []string{"1.1.1.1"})
		fillTestProviderClusterConfig(&input, master, nodeGroups)
		fillTestWithNatInstanceLayout(t, &input, settings)
		return input
	}

	assertWithNATInstance := func(t *testing.T, settings string, hasError bool, nodeGroups json.RawMessage) {
		input := getInput(t, settings, nodeGroups)
		assertValidation(t, true, input, hasError)
	}

	assertWithNATInstance(t, "", true, nil)
	assertWithNATInstance(t, `{}`, true, nil)
	assertWithNATInstance(t, `{"exporterAPIKey": "not security key"}`, true, nil)
	assertWithNATInstance(t, `{"internalSubnetID": "id", "internalSubnetCIDR": "127.0.0.1/24"}`, false, nil)
	assertWithNATInstance(t, `{"internalSubnetID": "id"}`, false, nil)
	assertWithNATInstance(t, `{"internalSubnetCIDR": "127.0.0.1/24"}`, false, nil)
	assertWithNATInstance(t,
		`{"internalSubnetCIDR": "127.0.0.1/24"}`,
		false,
		getTestNodeGroupsSpec(t, 1, []string{"1.1.1.1"}),
	)

	assertSkipValidationWithNATInstance := func(t *testing.T, settings string, nodeGroups json.RawMessage) {
		input := getInput(t, settings, nodeGroups)
		preparator := NewMetaConfigPreparator(true, log.NewSilentLogger(), "")

		err := preparator.Validate(context.TODO(), input)
		require.NoError(t, err)
	}

	assertSkipValidationWithNATInstance(t, "", nil)
	assertSkipValidationWithNATInstance(t, `{}`, nil)
	assertSkipValidationWithNATInstance(t, `{"exporterAPIKey": "not security key"}`, nil)
	assertSkipValidationWithNATInstance(t,
		`{"internalSubnetCIDR": "127.0.0.1/24"}`,
		getTestNodeGroupsSpec(t, 1, []string{"1.1.1.1"}),
	)
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	input := getTestInputForMaster(t, 1, []string{"1.1.1.1"})

	do := func() {
		preparator := NewMetaConfigPreparator(true, nil, "")
		_ = preparator.Validate(context.TODO(), input)
	}

	require.NotPanics(t, do)
}

func getTestInputForMaster(t *testing.T, replicas int, externalIPS []string) config.ProviderInput {
	input := config.ProviderInput{ClusterPrefix: "valid-prefix"}
	master := getTestMasterNodeGroupSpec(t, replicas, externalIPS)
	fillTestProviderClusterConfig(&input, master, nil)
	return input
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

func fillTestProviderClusterConfig(input *config.ProviderInput, master json.RawMessage, nodeGroups json.RawMessage) {
	input.ProviderClusterConfig = map[string]json.RawMessage{
		"masterNodeGroup": master,
	}
	if len(nodeGroups) > 0 {
		input.ProviderClusterConfig["nodeGroups"] = nodeGroups
	}
}

func fillTestWithNatInstanceLayout(t *testing.T, input *config.ProviderInput, settings string) {
	t.Helper()
	require.NotEmpty(t, input.ProviderClusterConfig)
	input.Layout = "with-nat-instance"
	if settings != "" {
		input.ProviderClusterConfig["withNATInstance"] = json.RawMessage(settings)
	}
}

func assertValidation(t *testing.T, _ bool, input config.ProviderInput, hasError bool) {
	preparator := NewMetaConfigPreparator(true, log.NewSilentLogger(), "bootstrap")

	err := preparator.Validate(context.TODO(), input)
	if hasError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
}
