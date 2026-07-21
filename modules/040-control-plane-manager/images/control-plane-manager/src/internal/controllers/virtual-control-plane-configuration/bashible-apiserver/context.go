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

	"sigs.k8s.io/yaml"
)

const (
	vcpPackagesProxyPort          = 443
	vcpPackagesProxyBootstrapPort = 80
)

type ContextInputParams struct {
	VCP                 *controlplanev1alpha1.VirtualControlPlane
	CA                  []byte
	JoinToken           string
	ClusterUUID         string
	APIHost             string
	PackagesHost        string
	APIServerProxyCerts ContextAPIServerProxyCerts
}

// contextInput is serialized with sigs.k8s.io/yaml (the same library the
// bashible-apiserver uses to read input.yaml), so only json tags matter.
type contextInput struct {
	Deckhouse               contextDeckhouse           `json:"deckhouse"`
	PodSubnetNodeCIDRPrefix string                     `json:"podSubnetNodeCIDRPrefix"`
	ClusterDomain           string                     `json:"clusterDomain"`
	ClusterDNSAddress       string                     `json:"clusterDNSAddress"`
	ClusterUUID             string                     `json:"clusterUUID"`
	BootstrapTokens         map[string]string          `json:"bootstrapTokens"`
	APIServerEndpoints      []string                   `json:"apiserverEndpoints"`
	ClusterMasterEndpoints  []contextMasterEndpoint    `json:"clusterMasterEndpoints"`
	APIServerProxyCerts     ContextAPIServerProxyCerts `json:"apiserverProxyCerts"`
	KubernetesCA            string                     `json:"kubernetesCA"`
	AllowedBundles          []string                   `json:"allowedBundles"`
	NodeGroups              []map[string]interface{}   `json:"nodeGroups"`
}

type ContextAPIServerProxyCerts struct {
	Crt string `json:"crt"`
	Key string `json:"key"`
}

type contextDeckhouse struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	Edition string `json:"edition"`
}
type contextMasterEndpoint struct {
	Address                string `json:"address"`
	KubeAPIPort            int    `json:"kubeApiPort"`
	RPPServerPort          int    `json:"rppServerPort"`
	RPPBootstrapServerPort int    `json:"rppBootstrapServerPort"`
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
	if p.APIServerProxyCerts.Crt == "" || p.APIServerProxyCerts.Key == "" {
		return "", fmt.Errorf("apiserverProxyCerts crt and key are required")
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
		APIServerProxyCerts: p.APIServerProxyCerts,
		KubernetesCA:        string(p.CA),
		AllowedBundles:      []string{"ubuntu-lts"},
		NodeGroups: []map[string]any{
			{
				"name":              "worker",
				"nodeType":          "Static",
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
