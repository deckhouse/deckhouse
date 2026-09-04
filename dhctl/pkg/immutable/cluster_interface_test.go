// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
)

// twoNICMachine is the machine the ladder exists for: the network dhctl pushes
// over, and the one the cluster lives on.
var twoNICMachine = &Inventory{Interfaces: []InventoryInterface{
	{Name: "eno1", Addresses: []string{"192.168.0.59/24"}},
	{Name: "eno2", Addresses: []string{"10.0.0.12/24"}},
}}

// The ladder, rung by rung. Each case is the first rung that answers, with every
// rung above it deliberately silent: a rung that starts answering out of turn
// changes which network the whole cluster is on.
func TestSelectClusterInterfaceWalksTheLadder(t *testing.T) {
	tests := []struct {
		name        string
		in          ClusterInterfaceInput
		wantName    string
		wantAddress string
		wantByPush  bool
	}{
		{
			name: "the operator marked one",
			in: ClusterInterfaceInput{
				Inventory:     twoNICMachine,
				Customization: customizationWithInterfaces(t, markedSecondNIC),
				// Both rungs below would answer eno1, and the mark still wins.
				InternalNetworkCIDRs: []string{"192.168.0.0/24"},
				PushAddress:          "192.168.0.59",
			},
			wantName: "eno2", wantAddress: "10.0.0.12",
		},
		{
			name: "the cluster names its networks",
			in: ClusterInterfaceInput{
				Inventory: twoNICMachine,
				// The order is the operator's: the second CIDR matches too, and the
				// first one listed is the answer.
				InternalNetworkCIDRs: []string{"10.0.0.0/16", "192.168.0.0/24"},
				PushAddress:          "192.168.0.59",
			},
			wantName: "eno2", wantAddress: "10.0.0.12",
		},
		{
			name: "the machine has a single interface with an address",
			in: ClusterInterfaceInput{
				Inventory: &Inventory{Interfaces: []InventoryInterface{
					{Name: "enp3s0", Addresses: []string{"192.168.0.59/24"}},
					{Name: "eno2"},
				}},
				PushAddress: "192.168.0.59",
			},
			wantName: "enp3s0", wantAddress: "192.168.0.59",
		},
		{
			name: "the operator described the interfaces and marked none",
			in: ClusterInterfaceInput{
				Inventory:     twoNICMachine,
				Customization: customizationWithInterfaces(t, describedSecondNICFirst),
				PushAddress:   "192.168.0.59",
			},
			wantName: "eno2", wantAddress: "10.0.0.12",
		},
		{
			name: "nothing names one, so the push address does",
			in: ClusterInterfaceInput{
				Inventory:   twoNICMachine,
				PushAddress: "192.168.0.59",
			},
			wantName: "eno1", wantAddress: "192.168.0.59", wantByPush: true,
		},
		{
			// A cloud machine that does not exist yet, or an image too old to serve
			// an inventory: the node resolves the interface itself.
			name:        "nothing is known about the machine",
			in:          ClusterInterfaceInput{PushAddress: "192.168.0.59"},
			wantName:    "",
			wantAddress: "192.168.0.59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chosen, err := SelectClusterInterface(tt.in)

			require.NoError(t, err)
			require.Equal(t, tt.wantName, chosen.Name)
			require.Equal(t, tt.wantAddress, chosen.Address)
			require.Equal(t, tt.wantByPush, chosen.ByPushAddress)
		})
	}
}

// A DHCP interface takes no address from the document, so the machine's own
// answer is what the CIDRs are matched against; a static one is matched against
// the address it is about to be given, which the machine does not hold yet.
func TestSelectClusterInterfaceReadsStaticAddressesFromTheDocument(t *testing.T) {
	chosen, err := SelectClusterInterface(ClusterInterfaceInput{
		Inventory: &Inventory{Interfaces: []InventoryInterface{
			{Name: "eno1", Addresses: []string{"192.168.0.59/24"}},
			{Name: "eno2", Addresses: []string{"192.168.1.7/24"}},
		}},
		Customization: customizationWithInterfaces(t, `
    - name: eno2
      dhcp: false
      addresses: ["10.0.0.12/24"]`),
		InternalNetworkCIDRs: []string{"10.0.0.0/16"},
		PushAddress:          "192.168.0.59",
	})

	require.NoError(t, err)
	require.Equal(t, "eno2", chosen.Name)
	require.Equal(t, "10.0.0.12", chosen.Address,
		"the address the document is about to give the interface, not the lease it holds now")
}

// Both refusals happen before anything is pushed: the machine is still untouched
// and the operator can still fix the input.
func TestSelectClusterInterfaceRefusals(t *testing.T) {
	t.Run("no address of the machine is in any cluster network", func(t *testing.T) {
		_, err := SelectClusterInterface(ClusterInterfaceInput{
			Inventory:            twoNICMachine,
			InternalNetworkCIDRs: []string{"172.16.0.0/16"},
			PushAddress:          "192.168.0.59",
		})

		require.ErrorContains(t, err, "172.16.0.0/16", "the network nothing answered")
		require.ErrorContains(t, err, "192.168.0.59", "and what the machine actually has")
		require.ErrorContains(t, err, "10.0.0.12")
		require.ErrorContains(t, err, "cluster: true", "the way out")
	})

	t.Run("the push address is on none of the interfaces", func(t *testing.T) {
		_, err := SelectClusterInterface(ClusterInterfaceInput{
			Inventory: twoNICMachine,
			// What a machine behind NAT looks like: dhctl reaches it at an address
			// the machine itself never sees.
			PushAddress: "203.0.113.7",
		})

		require.ErrorContains(t, err, "203.0.113.7")
		require.ErrorContains(t, err, "eno1", "the interfaces the machine has")
		require.ErrorContains(t, err, "eno2")
		require.ErrorContains(t, err, "internalNetworkCIDRs", "one way out")
		require.ErrorContains(t, err, "cluster: true", "the other")
	})

	t.Run("the marked interface is not on the machine", func(t *testing.T) {
		_, err := SelectClusterInterface(ClusterInterfaceInput{
			Inventory: twoNICMachine,
			Customization: customizationWithInterfaces(t, `
    - name: eno9
      dhcp: true
      cluster: true`),
			PushAddress: "192.168.0.59",
		})

		require.ErrorContains(t, err, "eno9")
		require.ErrorContains(t, err, "eno1", "and what the machine does have")
	})
}

// The node registers under a single address, so a document naming two cluster
// interfaces says nothing about which. Refused while it is still a document.
func TestCustomizationRefusesTwoClusterInterfaces(t *testing.T) {
	_, err := ParseCustomizations(t.Context(), []string{nodeConfigFor("master-0", `
  network:
    interfaces:
    - name: eno1
      dhcp: true
      cluster: true
    - name: eno2
      dhcp: true
      cluster: true
`)})

	require.ErrorContains(t, err, "master-0")
	require.ErrorContains(t, err, "eno1 and eno2")
}

// The rendered eth0 is a guess, and most hardware answers to enp3s0 or eno1
// instead. An operator who described no interface gets the machine's own.
func TestTheClusterInterfaceIsSynthesisedFromTheInventory(t *testing.T) {
	spec := nodeSpec{Network: network{Interfaces: []networkInterface{{Name: "eth0", DHCP: true}}}}

	markClusterInterface(&spec, ClusterInterface{Name: "enp3s0", Address: "192.168.0.59"}, false)

	require.Equal(t, []networkInterface{{Name: "enp3s0", DHCP: true, Cluster: true}}, spec.Network.Interfaces,
		"the guess is replaced by the NIC the machine reports")
}

// A document that describes the interfaces replaces the rendered list whole. The
// cluster interface has to be in it, or the NIC the cluster reaches the node on
// is never configured.
func TestTheClusterInterfaceIsAddedToADocumentThatDoesNotDescribeIt(t *testing.T) {
	spec := nodeSpec{Network: network{Interfaces: []networkInterface{
		{Name: "eno1", Addresses: []string{"192.168.0.59/24"}},
	}}}

	markClusterInterface(&spec, ClusterInterface{Name: "eno2", Address: "10.0.0.12"}, true)

	require.Equal(t, []networkInterface{
		{Name: "eno1", Addresses: []string{"192.168.0.59/24"}},
		{Name: "eno2", DHCP: true, Cluster: true},
	}, spec.Network.Interfaces)
}

// The last rung is a guess dhctl makes for the operator, and a guess nobody is
// told about is one nobody can correct.
func TestTheChoiceByPushAddressIsPrinted(t *testing.T) {
	var log bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&log, nil)))

	spec := nodeSpec{Network: network{Interfaces: []networkInterface{{Name: "eth0", DHCP: true}}}}
	address, err := applyClusterInterface(ctx, &spec, "master-0", ClusterInterfaceInput{
		Inventory:   twoNICMachine,
		PushAddress: "192.168.0.59",
	})

	require.NoError(t, err)
	require.Equal(t, "192.168.0.59", address)
	require.Contains(t, log.String(), "master-0")
	require.Contains(t, log.String(), "eno1", "the interface that was chosen")
	require.Contains(t, log.String(), "192.168.0.59", "and why")
}

// Every cloud calls the network its nodes talk over something else, and the
// value decides which interface a node that has to choose for itself takes.
func TestInternalNetworkCIDRsPerProvider(t *testing.T) {
	tests := []struct {
		provider string
		config   string
		want     []string
	}{
		{provider: "openstack", config: `{"standard": {"internalNetworkCIDR": "192.168.195.0/24"}}`, want: []string{"192.168.195.0/24"}},
		{provider: "openstack", config: `{"standardWithNoRouter": {"internalNetworkCIDR": "192.168.196.0/24"}}`, want: []string{"192.168.196.0/24"}},
		{provider: "openstack", config: `{"simple": {"externalNetworkName": "public"}}`, want: nil},
		{provider: "huaweicloud", config: `{"standard": {"internalNetworkCIDR": "192.168.199.0/24"}}`, want: []string{"192.168.199.0/24"}},
		{provider: "huaweicloud", config: `{"vpcPeering": {"internalNetworkCIDR": "192.168.198.0/24"}}`, want: []string{"192.168.198.0/24"}},
		{provider: "vsphere", config: `{"internalNetworkCIDR": "172.16.2.0/24"}`, want: []string{"172.16.2.0/24"}},
		{provider: "vcd", config: `{"internalNetworkCIDR": "172.16.3.0/24"}`, want: []string{"172.16.3.0/24"}},
		{provider: "yandex", config: `{"nodeNetworkCIDR": "10.241.32.0/24"}`, want: []string{"10.241.32.0/24"}},
		{provider: "dynamix", config: `{"nodeNetworkCIDR": "10.241.33.0/24"}`, want: []string{"10.241.33.0/24"}},
		{provider: "aws", config: `{"nodeNetworkCIDR": "172.16.0.0/22"}`, want: []string{"172.16.0.0/22"}},
		{provider: "aws", config: `{"vpcNetworkCIDR": "172.16.0.0/16"}`, want: nil},
		{provider: "azure", config: `{"subnetCIDR": "10.0.0.0/24"}`, want: []string{"10.0.0.0/24"}},
		{provider: "gcp", config: `{"subnetworkCIDR": "10.36.0.0/24"}`, want: []string{"10.36.0.0/24"}},
		{provider: "dvp", config: `{"virtualMachine": {}}`, want: nil},
		{provider: "zvirt", config: `{"clusterID": "x"}`, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.provider+" "+tt.config, func(t *testing.T) {
			var providerConfig map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tt.config), &providerConfig))

			cidrs, err := clusterInternalNetworkCIDRs(&config.MetaConfig{
				ClusterType:           config.CloudClusterType,
				ProviderName:          tt.provider,
				ProviderClusterConfig: providerConfig,
			})

			require.NoError(t, err)
			require.Equal(t, tt.want, cidrs)
		})
	}
}

// A static cluster names its networks itself, and dhctl must put them in every
// document: they are what a node with several NICs decides by.
func TestInternalNetworkCIDRsReachTheDocument(t *testing.T) {
	metaConfig := testMetaConfig(t)
	metaConfig.ClusterType = config.StaticClusterType
	metaConfig.StaticClusterConfig = map[string]json.RawMessage{
		"internalNetworkCIDRs": json.RawMessage(`["10.0.0.0/16","192.168.0.0/24"]`),
	}

	nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{NodeName: "master-0", MetaConfig: metaConfig})

	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.0/16", "192.168.0.0/24"}, nodeConfig.Spec.InternalNetworkCIDRs)
}

// Converge builds a payload with nothing known about the machine: no document,
// no inventory, no address. The cluster networks still have to reach it, because
// on that path they are all the node has to choose an interface by.
func TestTheJoinPayloadOfAnUnknownMachineCarriesTheClusterNetworks(t *testing.T) {
	metaConfig := testMetaConfig(t)
	metaConfig.ProviderName = "openstack"
	metaConfig.ProviderClusterConfig["standard"] = json.RawMessage(`{"internalNetworkCIDR": "192.168.195.0/24"}`)

	payload, nodeConfig, err := BuildJoinPayload(t.Context(), JoinPayloadInput{
		NodeName:           "worker-0",
		MetaConfig:         metaConfig,
		CACert:             "dGVzdC1jYQ==",
		BootstrapToken:     immutabletest.BootstrapToken,
		APIServerEndpoints: []string{"https://10.0.0.11:6443"},
		NodeGroupName:      "worker",
	})

	require.NoError(t, err)
	require.Contains(t, string(nodeConfig), "192.168.195.0/24")
	require.NotContains(t, decodePayload(t, payload), "cluster: true",
		"nothing here knows the machine's interfaces, so the node resolves them itself")
}

// The bearer authorises reading one machine's progress. Minted per document, so
// a machine only answers the installer that configured it.
func TestEveryDocumentCarriesItsOwnStatusToken(t *testing.T) {
	first, err := buildNodeConfig(t.Context(), nodeConfigInput{NodeName: "master-0", MetaConfig: testMetaConfig(t)})
	require.NoError(t, err)
	second, err := buildNodeConfig(t.Context(), nodeConfigInput{NodeName: "master-0", MetaConfig: testMetaConfig(t)})
	require.NoError(t, err)

	require.Len(t, first.Spec.StatusToken, 2*statusTokenBytes, "32 bytes, hex")
	require.NotEqual(t, first.Spec.StatusToken, second.Spec.StatusToken)
	require.Equal(t, strings.ToLower(first.Spec.StatusToken), first.Spec.StatusToken)
}

// The token travels in the document and nowhere else, so the wait reads it back
// out of the very bytes the machine was handed.
func TestStatusTokenOfReadsTheDocument(t *testing.T) {
	nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{NodeName: "master-0", MetaConfig: testMetaConfig(t)})
	require.NoError(t, err)

	document, err := buildDocumentStreamFor(nodeConfig)
	require.NoError(t, err)

	require.Equal(t, nodeConfig.Spec.StatusToken, StatusTokenOf(document))
	require.Empty(t, StatusTokenOf([]byte("not a document")))
}

func buildDocumentStreamFor(nodeConfig *nodeConfig) ([]byte, error) {
	_, document, err := buildDocumentStream(nodeConfig, nil)
	return document, err
}

const markedSecondNIC = `
    - name: eno1
      dhcp: true
    - name: eno2
      dhcp: true
      cluster: true`

const describedSecondNICFirst = `
    - name: eno2
      dhcp: true
    - name: eno1
      dhcp: true`

// customizationWithInterfaces builds a customization the way a run does — from a
// document — so the parse the operator's input goes through is exercised too.
func customizationWithInterfaces(t *testing.T, interfaces string) *Customization {
	t.Helper()

	parsed, err := ParseCustomizations(t.Context(), []string{nodeConfigFor("master-0", `
  network:
    interfaces:`+interfaces+"\n")})
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	return &parsed[0]
}
