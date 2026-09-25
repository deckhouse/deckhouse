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

package checks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

func cloudNetworkMetaConfig(pod, service string, providerConfig map[string]string) *config.MetaConfig {
	provider := map[string]json.RawMessage{}
	for key, value := range providerConfig {
		provider[key] = json.RawMessage(value)
	}
	return &config.MetaConfig{
		ProviderName: "yandex",
		ClusterConfig: map[string]json.RawMessage{
			"podSubnetCIDR":     json.RawMessage(`"` + pod + `"`),
			"serviceSubnetCIDR": json.RawMessage(`"` + service + `"`),
		},
		ProviderClusterConfig: provider,
	}
}

// TestCloudNodeNetworkCIDRIntersection covers the failure nothing checked before: on a cloud
// cluster an overlap between the pod subnet and the network the nodes are created on is only
// discovered after terraform has run, as a Deckhouse readiness timeout with nothing pointing at
// the subnets.
func TestCloudNodeNetworkCIDRIntersection(t *testing.T) {
	tests := []struct {
		name           string
		metaConfig     *config.MetaConfig
		notApplicable  bool
		wantErrContain string
	}{
		{
			name: "the node network is far from the cluster subnets",
			metaConfig: cloudNetworkMetaConfig("10.111.0.0/16", "10.222.0.0/16", map[string]string{
				"nodeNetworkCIDR": `"192.168.0.0/24"`,
			}),
		},
		{
			name: "the pod subnet contains the node network",
			metaConfig: cloudNetworkMetaConfig("10.111.0.0/16", "10.222.0.0/16", map[string]string{
				"nodeNetworkCIDR": `"10.111.32.0/24"`,
			}),
			wantErrContain: "podSubnetCIDR 10.111.0.0/16 overlaps YandexClusterConfiguration.nodeNetworkCIDR 10.111.32.0/24",
		},
		{
			name: "the service subnet contains the node network",
			metaConfig: cloudNetworkMetaConfig("10.111.0.0/16", "10.222.0.0/16", map[string]string{
				"nodeNetworkCIDR": `"10.222.32.0/24"`,
			}),
			wantErrContain: "serviceSubnetCIDR 10.222.0.0/16 overlaps YandexClusterConfiguration.nodeNetworkCIDR 10.222.32.0/24",
		},
		{
			// AWS declares two, and both are compared.
			name: "the wider VPC range is compared as well",
			metaConfig: cloudNetworkMetaConfig("172.16.0.0/16", "10.222.0.0/16", map[string]string{
				"nodeNetworkCIDR": `"192.168.0.0/24"`,
				"vpcNetworkCIDR":  `"172.16.0.0/16"`,
			}),
			wantErrContain: "podSubnetCIDR 172.16.0.0/16 overlaps YandexClusterConfiguration.vpcNetworkCIDR 172.16.0.0/16",
		},
		{
			// OpenStack and Huawei keep it inside the layout section.
			name: "a CIDR nested in the layout section is found",
			metaConfig: cloudNetworkMetaConfig("192.168.195.0/24", "10.222.0.0/16", map[string]string{
				"standard": `{"internalNetworkCIDR": "192.168.195.0/24"}`,
			}),
			wantErrContain: "overlaps YandexClusterConfiguration.standard.internalNetworkCIDR 192.168.195.0/24",
		},
		{
			name: "a provider configuration with no network declared",
			metaConfig: cloudNetworkMetaConfig("10.111.0.0/16", "10.222.0.0/16", map[string]string{
				"layout": `"Standard"`,
			}),
			notApplicable: true,
		},
		{
			name:          "no provider configuration at all",
			metaConfig:    cloudNetworkMetaConfig("10.111.0.0/16", "10.222.0.0/16", nil),
			notApplicable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CloudNodeNetworkCIDRIntersectionCheck{MetaConfig: tt.metaConfig}
			detail, err := check.Run(t.Context())

			switch {
			case tt.wantErrContain != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
			case tt.notApplicable:
				assert.ErrorIs(t, err, preflight.ErrNotApplicable)
			default:
				require.NoError(t, err)
				assert.Contains(t, detail, "do not overlap")
			}
		})
	}
}
