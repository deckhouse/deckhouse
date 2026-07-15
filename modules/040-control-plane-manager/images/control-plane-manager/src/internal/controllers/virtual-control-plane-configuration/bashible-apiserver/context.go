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
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
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

func BuildContextInputYAML(p ContextInputParams) (string, error) {
	if p.JoinToken == "" {
		return "", fmt.Errorf("join token is required")
	}

	clusterUUID := p.ClusterUUID
	if clusterUUID == "" {
		clusterUUID = "00000000-0000-0000-0000-000000000000"
	}

	apiEndpoint := fmt.Sprintf("%s:6443", p.APIHost)
	ca := string(p.CA)

	input := fmt.Sprintf(`
	deckhouse:
	  channel: "unknown"
	  version: "vcp"
	  edition: "unknown"
	podSubnetNodeCIDRPrefix: "24"
	clusterDomain: %q
	clusterDNSAddress: "10.96.0.10"
	clusterUUID: %q
	bootstrapTokens:
	  worker: %q
	apiserverEndpoints:
	  - %q
	clusterMasterEndpoints:
	  - address: %q
		kubeApiPort: 6443
		rppServerPort: %d
		rppBootstrapServerPort: %d
	  - address: %q
		kubeApiPort: 6443
		rppServerPort: %d
		rppBootstrapServerPort: %d
	kubernetesCA: |
	%s
	allowedBundles:
	  - ubuntu-lts
	nodeGroups:
	  - name: worker
	nodeStatusUpdateFrequency: 0
	`,
		constants.DefaultTenantClusterDomain,
		clusterUUID,
		p.JoinToken,
		apiEndpoint,
		p.APIHost, vcpPackagesProxyPort, vcpPackagesProxyBootstrapPort,
		p.PackagesHost, vcpPackagesProxyPort, vcpPackagesProxyBootstrapPort,
		indentYAML(ca, 2),
	)

	return input, nil
}

func indentYAML(s string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
