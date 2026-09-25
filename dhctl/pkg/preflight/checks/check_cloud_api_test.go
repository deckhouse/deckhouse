// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestCloudAPIEndpointPerProvider covers the endpoint table. Two of the eleven providers used to
// be here; for the other nine the check found no endpoint, returned nil, and the runner printed a
// ✓ for a question it had never asked.
func TestCloudAPIEndpointPerProvider(t *testing.T) {
	// A kubeconfig naming one cluster, as DVP carries it.
	dvpKubeconfig := base64.StdEncoding.EncodeToString([]byte(`
apiVersion: v1
kind: Config
clusters:
- name: dvp
  cluster:
    server: https://dvp.example.com:6443
`))

	tests := []struct {
		name               string
		providerName       string
		providerConfigJSON string
		expectedURL        string
		expectedField      string
	}{
		{
			name:         "OpenStack: the authURL field",
			providerName: "openstack",
			providerConfigJSON: `{
				"authURL": "https://openstack.example.com/v3/auth",
				"domainName": "provider.local",
				"tenantID": "tenantID",
				"username": "username",
				"password": "password",
				"region": "eu-3"
			}`,
			expectedURL:   "https://openstack.example.com/v3/auth",
			expectedField: "OpenStackClusterConfiguration.provider.authURL",
		},
		{
			name:         "vSphere: the server field",
			providerName: "vsphere",
			providerConfigJSON: `{
				"server": "https://vsphere.example.com/sdk",
				"username": "vsphereUser",
				"password": "vspherePass"
			}`,
			expectedURL:   "https://vsphere.example.com/sdk",
			expectedField: "VsphereClusterConfiguration.provider.server",
		},
		{
			name:         "vSphere: a bare host gets a scheme",
			providerName: "vsphere",
			providerConfigJSON: `{
				"server": "vcenter.bob.com",
				"username": "vsphereUser",
				"password": "vspherePass"
			}`,
			expectedURL:   "https://vcenter.bob.com",
			expectedField: "VsphereClusterConfiguration.provider.server",
		},
		{
			name:               "VCD: the server field",
			providerName:       "vcd",
			providerConfigJSON: `{"server": "vcd.example.com", "username": "u", "password": "p"}`,
			expectedURL:        "https://vcd.example.com",
			expectedField:      "VCDClusterConfiguration.provider.server",
		},
		{
			name:               "zVirt: the server field",
			providerName:       "zvirt",
			providerConfigJSON: `{"server": "https://zvirt.example.com/ovirt-engine/api"}`,
			expectedURL:        "https://zvirt.example.com/ovirt-engine/api",
			expectedField:      "ZvirtClusterConfiguration.provider.server",
		},
		{
			name:               "Dynamix: the controllerUrl field",
			providerName:       "dynamix",
			providerConfigJSON: `{"controllerUrl": "https://dynamix.example.com"}`,
			expectedURL:        "https://dynamix.example.com",
			expectedField:      "DynamixClusterConfiguration.provider.controllerUrl",
		},
		{
			name:               "Huawei Cloud: the authURL field",
			providerName:       "huaweicloud",
			providerConfigJSON: `{"authURL": "https://iam.eu-west-0.myhuaweicloud.com/v3"}`,
			expectedURL:        "https://iam.eu-west-0.myhuaweicloud.com/v3",
			expectedField:      "HuaweiCloudClusterConfiguration.provider.authURL",
		},
		{
			name:               "AWS: the regional EC2 endpoint",
			providerName:       "aws",
			providerConfigJSON: `{"region": "eu-central-1", "providerAccessKeyId": "id"}`,
			expectedURL:        "https://ec2.eu-central-1.amazonaws.com",
			expectedField:      "AWSClusterConfiguration.provider.region",
		},
		{
			name:               "GCP: the public endpoint",
			providerName:       "gcp",
			providerConfigJSON: `{"region": "europe-west4"}`,
			expectedURL:        "https://compute.googleapis.com",
			expectedField:      "the provider's public API endpoint",
		},
		{
			name:               "Azure: the public endpoint",
			providerName:       "azure",
			providerConfigJSON: `{"subscriptionId": "sub"}`,
			expectedURL:        "https://management.azure.com",
			expectedField:      "the provider's public API endpoint",
		},
		{
			name:               "Yandex: the public endpoint",
			providerName:       "yandex",
			providerConfigJSON: `{"cloudID": "cloud"}`,
			expectedURL:        "https://api.cloud.yandex.net",
			expectedField:      "the provider's public API endpoint",
		},
		{
			name:               "DVP: the server of the kubeconfig it is given",
			providerName:       "dvp",
			providerConfigJSON: `{"kubeconfigDataBase64": "` + dvpKubeconfig + `", "namespace": "default"}`,
			expectedURL:        "https://dvp.example.com:6443",
			expectedField:      "DVPClusterConfiguration.provider.kubeconfigDataBase64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := require.New(t)
			check := CloudAPICheck{MetaConfig: metaConfigForProvider(t, tt.providerName, tt.providerConfigJSON)}

			cloudAPIConfig, err := check.endpoint()
			s.NoError(err)
			s.NotNil(cloudAPIConfig, "an endpoint must be known for %q", tt.providerName)
			s.Equal(tt.expectedURL, cloudAPIConfig.URL.String())
			s.Equal(tt.expectedField, cloudAPIConfig.Field, "the failure has to name what to edit")
		})
	}
}

// TestCloudAPIUnknownProviderIsNotApplicable: a provider with no known endpoint is a question the
// check cannot ask, which is not the same as a question it asked and liked the answer to.
func TestCloudAPIUnknownProviderIsNotApplicable(t *testing.T) {
	check := CloudAPICheck{MetaConfig: metaConfigForProvider(t, "someprivatecloud", `{"server": "x"}`)}

	_, err := check.Run(t.Context())
	require.ErrorIs(t, err, preflight.ErrNotApplicable)
	require.Contains(t, err.Error(), "someprivatecloud")
}

// TestCloudAPIMalformedProviderConfigIsAFailure: a kubeconfig that does not parse is a mistake in
// the configuration, and saying so beats reporting the cloud API as unreachable.
func TestCloudAPIMalformedProviderConfigIsAFailure(t *testing.T) {
	check := CloudAPICheck{MetaConfig: metaConfigForProvider(t, "dvp", `{"kubeconfigDataBase64": "not base64!"}`)}

	_, err := check.Run(t.Context())
	require.Error(t, err)
	require.NotErrorIs(t, err, preflight.ErrNotApplicable)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	require.Contains(t, failure.Observed, "kubeconfigDataBase64")
}

func metaConfigForProvider(t *testing.T, providerName, providerConfigJSON string) *config.MetaConfig {
	t.Helper()

	const clusterConfigYAML = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
`
	metaConfig, err := config.ParseConfigFromData(t.Context(), clusterConfigYAML, config.DummyValidatorProvider(), &options.New().Global)
	require.NoError(t, err)

	metaConfig.ProviderName = providerName
	metaConfig.ProviderClusterConfig = map[string]json.RawMessage{
		"provider": json.RawMessage(providerConfigJSON),
	}
	return metaConfig
}
