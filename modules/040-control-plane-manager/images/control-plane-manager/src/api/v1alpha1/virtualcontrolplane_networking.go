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

package v1alpha1

import (
	"fmt"
	"net"

	utilnet "k8s.io/utils/net"
)

// clusterDNSIndex is the offset of the tenant DNS Service ClusterIP within ServiceSubnetCIDR,
// mirroring the convention a normal Deckhouse cluster (and kubeadm) uses: 10.96.0.10 for the
// default 10.96.0.0/12 range.
const clusterDNSIndex = 10

// ClusterDNSAddress returns the tenant DNS Service ClusterIP: the 10th address of
// ServiceSubnetCIDR. It is always derived, never stored as an independent field, so it cannot
// drift from the range the tenant apiserver actually serves — the drift that made the former
// constants.DefaultTenantClusterDNS a latent bug.
//
// The CRD schema is the validation gate for ServiceSubnetCIDR (pattern + size bounds), so a value
// reaching here from the API server always parses; the error return covers hand-built objects in
// tests and any future path that does not go through admission.
func (n VirtualControlPlaneNetworking) ClusterDNSAddress() (string, error) {
	ip, network, err := net.ParseCIDR(n.ServiceSubnetCIDR)
	if err != nil {
		return "", fmt.Errorf("parse spec.networking.serviceSubnetCIDR %q: %w", n.ServiceSubnetCIDR, err)
	}
	if ip.To4() == nil {
		return "", fmt.Errorf("spec.networking.serviceSubnetCIDR %q is not IPv4", n.ServiceSubnetCIDR)
	}

	dnsIP, err := utilnet.GetIndexedIP(network, clusterDNSIndex)
	if err != nil {
		return "", fmt.Errorf("derive cluster DNS address from spec.networking.serviceSubnetCIDR %q: %w", n.ServiceSubnetCIDR, err)
	}

	return dnsIP.String(), nil
}
