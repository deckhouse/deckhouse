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

package preflight

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestGetCloudApiURLFromMetaConfig(t *testing.T) {
	tests := []struct {
		name               string
		providerName       string
		providerConfigJSON string
		expectedURL        string
	}{
		{
			name:         "OpenStack provider",
			providerName: "openstack",
			providerConfigJSON: `{
				"authURL": "https://openstack.example.com/v3/auth",
				"domainName": "provider.local",
				"tenantID": "tenantID",
				"username": "username",
				"password": "password",
				"region": "eu-3"
			}`,
			expectedURL: "https://openstack.example.com/v3/auth",
		},
		{
			name:         "vSphere provider",
			providerName: "vsphere",
			providerConfigJSON: `{
				"server": "https://vsphere.example.com/sdk",
				"username": "vsphereUser",
				"password": "vspherePass"
			}`,
			expectedURL: "https://vsphere.example.com/sdk",
		},
		{
			name:         "vSphere provider",
			providerName: "vsphere",
			providerConfigJSON: `{
				"server": "vcenter.bob.com",
				"username": "vsphereUser",
				"password": "vspherePass"
			}`,
			expectedURL: "https://vcenter.bob.com",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := require.New(t)
			clusterConfigYAML := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
`
			metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfigYAML, config.DummyPreparatorProvider())
			s.NoError(err)
			metaConfig.ProviderName = tt.providerName
			metaConfig.ProviderClusterConfig = map[string]json.RawMessage{
				"provider": json.RawMessage(tt.providerConfigJSON),
			}
			s.Equal(tt.providerName, metaConfig.ProviderName)
			cloudApiConfig, err := getCloudApiConfigFromMetaConfig(metaConfig)
			t.Logf("cloudApiConfig: %+v, error: %v", cloudApiConfig, err)
			s.NoError(err, "getCloudApiConfigFromMetaConfig must be not nil")
			s.NotNil(cloudApiConfig, "cloudApiConfig must be not nil")
			cloudApiURL := cloudApiConfig.URL.String()
			s.NoError(err)
			s.Equal(tt.expectedURL, cloudApiURL)
		})
	}
}
