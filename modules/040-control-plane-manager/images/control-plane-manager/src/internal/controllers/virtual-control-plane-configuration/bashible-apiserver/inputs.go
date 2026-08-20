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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"sigs.k8s.io/yaml"
)

const (
	// ExternalInputsVersion is the format of inputs.yaml. A tenant that does not recognise the
	// version assembles nothing, so a host running ahead of a tenant switches that tenant off
	// instead of handing it a document it would misread.
	ExternalInputsVersion = 1

	ExternalInputsSecretName = "bashible-external-inputs"
	ExternalInputsSecretKey  = "inputs.yaml"

	// ExternalInputsRevisionAnnotation records which inputs the tenant was last handed, so an
	// incident responder can tell a stale Secret from a current one without decoding it.
	ExternalInputsRevisionAnnotation = "control-plane.deckhouse.io/inputs-revision"

	vcpPackagesProxyPort          = 443
	vcpPackagesProxyBootstrapPort = 80
)

// externalInputs is the host -> tenant contract: the parts of the bashible input.yaml a tenant
// cannot derive from its own cluster, because its control plane, its PKI and its node lifecycle
// all live in the parent. Everything absent from here (today: clusterDomain, and the optional
// cloudProvider/proxy/nodeStatusUpdateFrequency/allowedKubeletFeatureGates keys) the tenant reads
// from its own objects through go_lib/bashiblecontext.
//
// The json tags are the bashible input.yaml keys the values end up under, so the tenant overlays
// them by straight assignment and TestExternalInputsMatchContextInput can hold this document
// against the context the tenant is expected to produce.
type externalInputs struct {
	Version int `json:"version"`

	Deckhouse               inputsDeckhouse            `json:"deckhouse"`
	PodSubnetNodeCIDRPrefix string                     `json:"podSubnetNodeCIDRPrefix"`
	ClusterDNSAddress       string                     `json:"clusterDNSAddress"`
	ClusterUUID             string                     `json:"clusterUUID"`
	BootstrapTokens         map[string]string          `json:"bootstrapTokens"`
	APIServerEndpoints      []string                   `json:"apiserverEndpoints"`
	ClusterMasterEndpoints  []inputsMasterEndpoint     `json:"clusterMasterEndpoints"`
	APIServerProxyCerts     ContextAPIServerProxyCerts `json:"apiserverProxyCerts"`
	KubernetesCA            string                     `json:"kubernetesCA"`
	AllowedBundles          []string                   `json:"allowedBundles"`
	NodeGroups              []map[string]interface{}   `json:"nodeGroups"`
	PackagesProxy           map[string]interface{}     `json:"packagesProxy"`
}

type ExternalInputsParams struct {
	VCP                 *controlplanev1alpha1.VirtualControlPlane
	CA                  []byte
	JoinToken           string
	ClusterUUID         string
	APIHost             string
	PackagesHost        string
	RPPToken            string
	APIServerProxyCerts ContextAPIServerProxyCerts
}

// ContextAPIServerProxyCerts is the apiserverProxyCerts field of the bashible context. The pair
// is signed once and republished unchanged: every node keeps it in its api-proxy configuration.
type ContextAPIServerProxyCerts struct {
	Crt string `json:"crt"`
	Key string `json:"key"`
}

type inputsDeckhouse struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	Edition string `json:"edition"`
}

type inputsMasterEndpoint struct {
	Address                string `json:"address"`
	KubeAPIPort            int    `json:"kubeApiPort"`
	RPPServerPort          int    `json:"rppServerPort"`
	RPPBootstrapServerPort int    `json:"rppBootstrapServerPort"`
}

// BuildExternalInputsYAML renders inputs.yaml for the tenant. It is serialized with
// sigs.k8s.io/yaml (the same library bashible-apiserver reads input.yaml with), so only the json
// tags matter.
func BuildExternalInputsYAML(p ExternalInputsParams) (string, error) {
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
	// ClusterDNSAddress is derived from spec.networking.serviceSubnetCIDR rather than passed in,
	// so it can never disagree with the range the tenant apiserver serves.
	clusterDNSAddress, err := p.VCP.Spec.Networking.ClusterDNSAddress()
	if err != nil {
		return "", err
	}

	clusterUUID := p.ClusterUUID
	if clusterUUID == "" {
		clusterUUID = "00000000-0000-0000-0000-000000000000"
	}

	inputs := externalInputs{
		Version: ExternalInputsVersion,
		Deckhouse: inputsDeckhouse{
			Channel: "unknown",
			Version: "vcp",
			Edition: "unknown",
		},
		PodSubnetNodeCIDRPrefix: p.VCP.Spec.Networking.PodSubnetNodeCIDRPrefix,
		ClusterDNSAddress:       clusterDNSAddress,
		ClusterUUID:             clusterUUID,
		BootstrapTokens: map[string]string{
			"worker": p.JoinToken,
		},
		APIServerEndpoints: []string{
			fmt.Sprintf("%s:6443", p.APIHost),
		},
		ClusterMasterEndpoints: []inputsMasterEndpoint{
			{
				Address:     p.APIHost,
				KubeAPIPort: 6443,
			},
			{
				Address:                p.PackagesHost,
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
		PackagesProxy: map[string]any{
			"direct": true,
			"token":  p.RPPToken,
		},
	}

	out, err := yaml.Marshal(inputs)
	if err != nil {
		return "", fmt.Errorf("marshal bashible inputs.yaml: %w", err)
	}

	return string(out), nil
}

// ExternalInputsRevision is the value of ExternalInputsRevisionAnnotation.
func ExternalInputsRevision(inputsYAML string) string {
	sum := sha256.Sum256([]byte(inputsYAML))
	return hex.EncodeToString(sum[:])
}
