/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	schemaStore := config.NewSchemaStore(
		"../../../../../../candi/openapi",
		"../../../../../../candi/cloud-providers",
	)

	tests := map[string]struct {
		data        []byte
		errContains string
	}{
		"error, not a secret": {
			data:        []byte("kind: Pod\napiVersion: v1"),
			errContains: "object is not of type corev1.Secret: /v1, Kind=Pod",
		},
		"ok, cluster config": {
			data: []byte(fmt.Sprintf(
				d8ClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8ClusterConfiguration))),
			),
		},
		"error, cluster config": {
			data: []byte(fmt.Sprintf(
				d8ClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8ClusterConfigurationErr))),
			),
			errContains: "clusterType should be one of [Cloud Static]",
		},
		"ok, provider cluster config": {
			data: []byte(fmt.Sprintf(
				d8ProviderClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8ProviderClusterConfiguration))),
			),
		},
		"error, provider cluster config": {
			data: []byte(fmt.Sprintf(
				d8ProviderClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8ProviderClusterConfigurationErr))),
			),
			errContains: "layout should be one of [Standard WithoutNAT]",
		},
		"ok, static cluster config": {
			data: []byte(fmt.Sprintf(
				d8StaticClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8StaticClusterConfiguration))),
			),
		},
		"error, static cluster config": {
			data: []byte(fmt.Sprintf(
				d8StaticClusterConfigurationSecret,
				base64.StdEncoding.EncodeToString([]byte(d8StaticClusterConfigurationErr))),
			),
			errContains: ".unknownField is a forbidden property",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := validate(schemaStore, tt.data)
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

var (
	d8ClusterConfigurationSecret = `
{
	"apiVersion": "v1",
	"kind": "Secret",
	"metadata": {
		"name": "d8-cluster-configuration",
		"namespace": "kube-system"
	},
	"data":{
		"cluster-configuration.yaml": "%s"
	}
}`
	d8ClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"`
	d8ClusterConfigurationErr = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: unknown`
)

var (
	d8ProviderClusterConfigurationSecret = `
{
	"apiVersion": "v1",
	"kind": "Secret",
	"metadata": {
		"name": "d8-provider-cluster-configuration",
		"namespace": "kube-system"
	},
	"data":{
		"cloud-provider-cluster-configuration.yaml": "%s"
	}
}`
	d8ProviderClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: 'YjFnYnA2bHVybDBzbXA2Y2kzanMK'
  folderID: 'b1gsqe7ct9jtss0mlmid'
  serviceAccountJSON: |
    {"id": "ajeqlssun75pno7f46t7"}
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 8
    memory: 8192
    imageID: fd8li2lvvfc6bdj4c787
    externalIPAddresses:
    - "Auto"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: ssh-key`
	d8ProviderClusterConfigurationErr = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: unknown`
)

var (
	d8StaticClusterConfigurationSecret = `
{
	"apiVersion": "v1",
	"kind": "Secret",
	"metadata": {
		"name": "d8-static-cluster-configuration",
		"namespace": "kube-system"
	},
	"data":{
		"static-cluster-configuration.yaml": "%s"
	}
}`
	d8StaticClusterConfiguration = `
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0/24`
	d8StaticClusterConfigurationErr = `
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
unknownField: value`
)
