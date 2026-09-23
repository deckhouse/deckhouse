/*
Copyright 2026 Flant JSC

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

package common

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/network"
)

const (
	ClusterConfigSecretName      = "d8-cluster-configuration"
	ClusterConfigSecretNamespace = KubeSystemNamespace
	clusterConfigSecretKey       = "cluster-configuration.yaml"
)

// ClusterConfiguration is the subset of ClusterConfiguration used by node-controller.
type ClusterConfiguration struct {
	PodSubnetCIDR     string `json:"podSubnetCIDR"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	ClusterDomain     string `json:"clusterDomain"`
	Cloud             struct {
		Prefix string `json:"prefix"`
	} `json:"cloud"`
}

// ReadClusterConfiguration reads cluster-configuration.yaml. The Secret value is normally raw
// YAML after Kubernetes decodes data, but older installations may contain a double-encoded value.
func ReadClusterConfiguration(ctx context.Context, reader client.Reader) (ClusterConfiguration, error) {
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{
		Name: ClusterConfigSecretName, Namespace: ClusterConfigSecretNamespace,
	}, secret); err != nil {
		return ClusterConfiguration{}, fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return ClusterConfiguration{}, fmt.Errorf("cluster-configuration secret has no %s key", clusterConfigSecretKey)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		decoded = raw
	}

	configuration := ClusterConfiguration{}
	if err := sigsyaml.Unmarshal(decoded, &configuration); err != nil {
		return ClusterConfiguration{}, fmt.Errorf("unmarshal cluster configuration: %w", err)
	}

	// TODO: Remove when cluster-configuration is removed and use only ModuleConfig
	// ModuleConfig wins over these deprecated fields when set (see package network). The override
	// lives here rather than in each caller: the CAPI Cluster, the MCM machine class and the
	// provider templates render their subnets out of this one read, and a caller resolving them
	// its own way would describe a different cluster network than the control plane runs with.
	mcNetwork, err := network.FromModuleConfig(ctx, reader)
	if err != nil {
		return ClusterConfiguration{}, fmt.Errorf("resolve network settings: %w", err)
	}
	configuration.PodSubnetCIDR = cmp.Or(mcNetwork.PodSubnetCIDR, configuration.PodSubnetCIDR)
	configuration.ServiceSubnetCIDR = cmp.Or(mcNetwork.ServiceSubnetCIDR, configuration.ServiceSubnetCIDR)

	return configuration, nil
}
