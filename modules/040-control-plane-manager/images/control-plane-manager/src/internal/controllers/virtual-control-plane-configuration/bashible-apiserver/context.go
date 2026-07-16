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

package bashibleapiserver

import (
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"go.yaml.in/yaml/v2"
)

const (
	vcpPackagesProxyPort          = 443
	vcpPackagesProxyBootstrapPort = 80
)

type ContextInputParams struct {
	VCP          *controlplanev1alpha1.VirtualControlPlane
	CA           []byte
	JoinToken    string
	ClusterUUID  string
	APIHost      string
	PackagesHost string
}

type contextInput struct {
	Deckhouse               contextDeckhouse         `json:"deckhouse" yaml:"deckhouse"`
	PodSubnetNodeCIDRPrefix string                   `json:"podSubnetNodeCIDRPrefix" yaml:"podSubnetNodeCIDRPrefix"`
	ClusterDomain           string                   `json:"clusterDomain" yaml:"clusterDomain"`
	ClusterDNSAddress       string                   `json:"clusterDNSAddress" yaml:"clusterDNSAddress"`
	ClusterUUID             string                   `json:"clusterUUID" yaml:"clusterUUID"`
	BootstrapTokens         map[string]string        `json:"bootstrapTokens" yaml:"bootstrapTokens"`
	APIServerEndpoints      []string                 `json:"apiserverEndpoints" yaml:"apiserverEndpoints"`
	ClusterMasterEndpoints  []contextMasterEndpoint  `json:"clusterMasterEndpoints" yaml:"clusterMasterEndpoints"`
	KubernetesCA            string                   `json:"kubernetesCA" yaml:"kubernetesCA"`
	AllowedBundles          []string                 `json:"allowedBundles" yaml:"allowedBundles"`
	NodeGroups              []map[string]interface{} `json:"nodeGroups" yaml:"nodeGroups"`
}
type contextDeckhouse struct {
	Channel string `json:"channel" yaml:"channel"`
	Version string `json:"version" yaml:"version"`
	Edition string `json:"edition" yaml:"edition"`
}
type contextMasterEndpoint struct {
	Address                string `json:"address" yaml:"address"`
	KubeAPIPort            int    `json:"kubeApiPort" yaml:"kubeApiPort"`
	RPPServerPort          int    `json:"rppServerPort" yaml:"rppServerPort"`
	RPPBootstrapServerPort int    `json:"rppBootstrapServerPort" yaml:"rppBootstrapServerPort"`
}

func BuildContextInputYAML(p ContextInputParams) (string, error) {
	if p.JoinToken == "" {
		return "", fmt.Errorf("join token is required")
	}
	if p.VCP == nil {
		return "", fmt.Errorf("virtual control plane is required")
	}
	if p.VCP.Spec.KubernetesVersion == "" {
		return "", fmt.Errorf("kubernetes version is required")
	}

	clusterUUID := p.ClusterUUID
	if clusterUUID == "" {
		clusterUUID = "00000000-0000-0000-0000-000000000000"
	}

	input := contextInput{
		Deckhouse: contextDeckhouse{
			Channel: "unknown",
			Version: "vcp",
			Edition: "unknown",
		},
		PodSubnetNodeCIDRPrefix: "24",
		ClusterDomain:           constants.DefaultTenantClusterDomain,
		ClusterDNSAddress:       "10.96.0.10",
		ClusterUUID:             clusterUUID,
		BootstrapTokens: map[string]string{
			"worker": p.JoinToken,
		},
		APIServerEndpoints: []string{
			fmt.Sprintf("%s:6443", p.APIHost),
		},
		ClusterMasterEndpoints: []contextMasterEndpoint{
			{
				Address:                p.APIHost,
				KubeAPIPort:            6443,
				RPPServerPort:          vcpPackagesProxyPort,
				RPPBootstrapServerPort: vcpPackagesProxyBootstrapPort,
			},
			{
				Address:                p.PackagesHost,
				KubeAPIPort:            6443,
				RPPServerPort:          vcpPackagesProxyPort,
				RPPBootstrapServerPort: vcpPackagesProxyBootstrapPort,
			},
		},
		KubernetesCA:   string(p.CA),
		AllowedBundles: []string{"ubuntu-lts"},
		NodeGroups: []map[string]any{
			{
				"name":              "worker",
				"kubernetesVersion": p.VCP.Spec.KubernetesVersion,
				"cri": map[string]any{
					"type": "Containerd",
				},
			},
		},
	}

	out, err := yaml.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal bashible input.yaml: %w", err)
	}

	return string(out), nil
}
