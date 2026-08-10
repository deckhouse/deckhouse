// Copyright 2026 Flant JSC
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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/mocks"
)

func TestCheckClusterCIDRsAgainstHost(t *testing.T) {
	tests := []struct {
		name              string
		podCIDR           string
		serviceCIDR       string
		host              hostNetworkState
		wantError         string
		wantErrorContains string
	}{
		{
			name:        "no intersections",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Networks: []detectedNetwork{
					{
						CIDR:   "192.168.1.0/24",
						Source: "interface eth0",
					},
				},
			},
		},
		{
			name:        "service CIDR contains DNS server",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Addresses: []detectedAddress{
					{
						Address: "10.222.0.10",
						Source:  "DNS server",
					},
				},
			},
			wantError: "serviceSubnetCIDR 10.222.0.0/16 contains DNS server 10.222.0.10",
		},
		{
			name:        "pod CIDR intersects interface network",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Networks: []detectedNetwork{
					{
						CIDR:   "10.111.10.0/24",
						Source: "interface eth0",
					},
				},
			},
			wantError: "podSubnetCIDR 10.111.0.0/16 intersects with interface eth0 10.111.10.0/24",
		},
		{
			name:        "service CIDR intersects route",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Networks: []detectedNetwork{
					{
						CIDR:   "10.222.128.0/24",
						Source: "route via eth1",
					},
				},
			},
			wantError: "serviceSubnetCIDR 10.222.0.0/16 intersects with route via eth1 10.222.128.0/24",
		},
		{
			name:        "pod CIDR contains default gateway",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Addresses: []detectedAddress{
					{
						Address: "10.111.0.1",
						Source:  "default gateway",
					},
				},
			},
			wantError: "podSubnetCIDR 10.111.0.0/16 contains default gateway 10.111.0.1",
		},
		{
			name:        "IPv6 networks intersect",
			podCIDR:     "fd00:111::/64",
			serviceCIDR: "fd00:222::/112",
			host: hostNetworkState{
				Networks: []detectedNetwork{
					{
						CIDR:   "fd00:111::1000/120",
						Source: "interface eth0",
					},
				},
			},
			wantError: "podSubnetCIDR fd00:111::/64 intersects with interface eth0 fd00:111::1000/120",
		},
		{
			name:        "invalid discovered network",
			podCIDR:     "10.111.0.0/16",
			serviceCIDR: "10.222.0.0/16",
			host: hostNetworkState{
				Networks: []detectedNetwork{
					{
						CIDR:   "not-a-cidr",
						Source: "interface eth0",
					},
				},
			},
			wantErrorContains: `invalid CIDR "not-a-cidr" discovered from interface eth0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkClusterCIDRsAgainstHost(
				tt.podCIDR,
				tt.serviceCIDR,
				tt.host,
			)

			switch {
			case tt.wantError != "":
				assert.EqualError(t, err, tt.wantError)
			case tt.wantErrorContains != "":
				assert.ErrorContains(t, err, tt.wantErrorContains)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseIPAddresses(t *testing.T) {
	output := []byte(`[
		{
			"ifname": "eth0",
			"addr_info": [
				{
					"family": "inet",
					"local": "192.168.10.15",
					"prefixlen": 24
				},
				{
					"family": "inet6",
					"local": "fd00:10::15",
					"prefixlen": 64
				}
			]
		},
		{
			"ifname": "eth1",
			"addr_info": [
				{
					"family": "inet",
					"local": "172.16.5.10",
					"prefixlen": 16
				}
			]
		}
	]`)

	networks, err := parseIPAddresses(output)

	assert.NoError(t, err)
	assert.Equal(t, []detectedNetwork{
		{
			CIDR:   "192.168.10.0/24",
			Source: "interface eth0",
		},
		{
			CIDR:   "fd00:10::/64",
			Source: "interface eth0",
		},
		{
			CIDR:   "172.16.0.0/16",
			Source: "interface eth1",
		},
	}, networks)
}

func TestParseIPRoutes(t *testing.T) {
	output := []byte(`[
		{
			"dst": "default",
			"gateway": "192.168.10.1",
			"dev": "eth0"
		},
		{
			"dst": "10.50.0.0/16",
			"gateway": "192.168.10.2",
			"dev": "eth0"
		},
		{
			"dst": "203.0.113.42",
			"dev": "eth1"
		},
		{
			"dst": "fd00:50::/64",
			"dev": "eth2"
		}
	]`)

	state, err := parseIPRoutes(output)

	assert.NoError(t, err)
	assert.Equal(t, []detectedNetwork{
		{
			CIDR:   "10.50.0.0/16",
			Source: "route via eth0",
		},
		{
			CIDR:   "203.0.113.42/32",
			Source: "route via eth1",
		},
		{
			CIDR:   "fd00:50::/64",
			Source: "route via eth2",
		},
	}, state.Networks)

	assert.Equal(t, []detectedAddress{
		{
			Address: "192.168.10.1",
			Source:  "default gateway via eth0",
		},
	}, state.Addresses)
}

func TestParseResolvConf(t *testing.T) {
	output := []byte(`
# Generated by systemd-resolved
search example.internal
nameserver 10.222.0.10
nameserver 1.1.1.1 # public DNS
nameserver fd00:53::1
options edns0 trust-ad
`)

	addresses, err := parseResolvConf(output)

	assert.NoError(t, err)
	assert.Equal(t, []detectedAddress{
		{
			Address: "10.222.0.10",
			Source:  "DNS server",
		},
		{
			Address: "1.1.1.1",
			Source:  "DNS server",
		},
		{
			Address: "fd00:53::1",
			Source:  "DNS server",
		},
	}, addresses)
}

func TestHostNetworkCIDRIntersectionCheck_Run(t *testing.T) {
	nodeInterface := &mocks.MockNodeInterface{}

	commands := []*mocks.MockCommand{
		expectHostCommand(
			nodeInterface,
			"ip",
			[]string{"-j", "address", "show"},
			`[
				{
					"ifname": "eth0",
					"addr_info": [
						{
							"family": "inet",
							"local": "192.168.10.15",
							"prefixlen": 24
						}
					]
				}
			]`,
		),
		expectHostCommand(
			nodeInterface,
			"ip",
			[]string{
				"-j",
				"-4",
				"route",
				"show",
				"table",
				"all",
			},
			`[
				{
					"dst": "default",
					"gateway": "192.168.10.1",
					"dev": "eth0"
				},
				{
					"dst": "192.168.10.0/24",
					"dev": "eth0"
				}
			]`,
		),
		expectHostCommand(
			nodeInterface,
			"ip",
			[]string{
				"-j",
				"-6",
				"route",
				"show",
				"table",
				"all",
			},
			`[]`,
		),
		expectHostCommand(
			nodeInterface,
			"cat",
			[]string{"/etc/resolv.conf"},
			`
				search parent-cluster.local
				nameserver 10.222.0.10
			`,
		),
	}

	check := HostNetworkCIDRIntersectionCheck{
		MetaConfig: &config.MetaConfig{
			ClusterConfig: map[string]json.RawMessage{
				"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
				"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
			},
		},
		NodeInterface: nodeInterface,
	}

	err := check.Run(t.Context())

	assert.EqualError(
		t,
		err,
		"serviceSubnetCIDR 10.222.0.0/16 contains DNS server 10.222.0.10",
	)

	nodeInterface.AssertExpectations(t)
	for _, command := range commands {
		command.AssertExpectations(t)
	}
}

func expectHostCommand(
	nodeInterface *mocks.MockNodeInterface,
	name string,
	args []string,
	stdout string,
) *mocks.MockCommand {
	command := &mocks.MockCommand{}

	nodeInterface.
		On("Command", name, args).
		Return(command).
		Once()

	command.
		On("Output", mock.Anything).
		Return([]byte(stdout), []byte{}, nil).
		Once()

	return command
}

func TestHostCommandOutputReturnsCommandError(t *testing.T) {
	nodeInterface := &mocks.MockNodeInterface{}
	command := &mocks.MockCommand{}

	nodeInterface.
		On(
			"Command",
			"ip",
			[]string{"-j", "address", "show"},
		).
		Return(command).
		Once()

	command.
		On("Output", mock.Anything).
		Return(
			[]byte{},
			[]byte("ip: command not found"),
			errors.New("exit status 127"),
		).
		Once()

	output, err := hostCommandOutput(
		t.Context(),
		nodeInterface,
		"ip",
		"-j",
		"address",
		"show",
	)

	assert.Nil(t, output)
	assert.ErrorContains(t, err, "execute host command ip")
	assert.ErrorContains(t, err, "exit status 127")
	assert.ErrorContains(t, err, "ip: command not found")

	nodeInterface.AssertExpectations(t)
	command.AssertExpectations(t)
}
