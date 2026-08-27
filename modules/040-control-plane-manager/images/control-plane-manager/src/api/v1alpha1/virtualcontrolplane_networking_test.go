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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterDNSAddress(t *testing.T) {
	for _, tc := range []struct {
		name              string
		serviceSubnetCIDR string
		want              string
	}{
		{
			// The value the tenant control plane used when it was a hardcoded constant. Locking it
			// in keeps already-provisioned tenants byte-identical.
			name:              "default range yields the former DefaultTenantClusterDNS",
			serviceSubnetCIDR: "10.96.0.0/12",
			want:              "10.96.0.10",
		},
		{
			name:              "smallest permitted range",
			serviceSubnetCIDR: "192.168.1.0/24",
			want:              "192.168.1.10",
		},
		{
			name:              "largest permitted range",
			serviceSubnetCIDR: "172.16.0.0/12",
			want:              "172.16.0.10",
		},
		{
			// The 10th address must be counted from the network address, not by bumping the last
			// octet — this range makes the difference visible.
			name:              "offset is counted from the network address",
			serviceSubnetCIDR: "10.0.1.0/24",
			want:              "10.0.1.10",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := VirtualControlPlaneNetworking{ServiceSubnetCIDR: tc.serviceSubnetCIDR}.ClusterDNSAddress()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	for _, tc := range []struct {
		name              string
		serviceSubnetCIDR string
	}{
		{name: "empty", serviceSubnetCIDR: ""},
		{name: "not a CIDR", serviceSubnetCIDR: "10.96.0.0"},
		{name: "garbage", serviceSubnetCIDR: "nonsense"},
		{name: "IPv6 is rejected rather than mishandled", serviceSubnetCIDR: "fd00::/108"},
		{name: "range too small to hold the 10th address", serviceSubnetCIDR: "10.96.0.0/30"},
	} {
		t.Run(tc.name+" errors", func(t *testing.T) {
			_, err := VirtualControlPlaneNetworking{ServiceSubnetCIDR: tc.serviceSubnetCIDR}.ClusterDNSAddress()
			assert.Error(t, err)
		})
	}
}
