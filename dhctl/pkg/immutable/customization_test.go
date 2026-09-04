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
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestCustomizationReplacesNetworkKeepingHostname(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    interfaces:
    - name: eno1
      dhcp: false
      addresses: ["10.0.0.11/24"]
      gateway: 10.0.0.1
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	require.Equal(t, "master-0", parsed[0].NodeName)

	spec := nodeSpec{Network: network{
		Hostname:   "master-0",
		Interfaces: []networkInterface{{Name: "eth0", DHCP: true}},
	}}
	applyCustomization(&spec, parsed[0])

	require.Equal(t, "master-0", spec.Network.Hostname)
	require.Equal(t, []networkInterface{{
		Name: "eno1", DHCP: false, Addresses: []string{"10.0.0.11/24"}, Gateway: "10.0.0.1",
	}}, spec.Network.Interfaces)
}

func TestCustomizationRefusesNodeGroupSettings(t *testing.T) {
	document := nodeConfigFor("master-0", `
  kubelet:
    maxPods: 250
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "kubelet",
		"the whole kubelet section is a NodeGroup setting now that nodeIP is gone from it")
	require.ErrorContains(t, err, "network and storage")
}

// The refusal must not name the NodeGroup as the single reason: spec.network.ntp
// is in the NodeConfig CRD and node-controller keeps spec.network whole, so
// "everything else comes from the NodeGroup" sends the operator looking for a
// field the NodeGroup does not have either.
func TestCustomizationRefusesNTPWithoutBlamingTheNodeGroup(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    ntp:
      servers: ["10.0.0.1"]
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "ntp")
	require.NotContains(t, err.Error(), "comes from the NodeGroup",
		"ntp is not a NodeGroup setting; it is simply not a field dhctl puts in the payload")
}

// bindTo and mode on the rendered mount have no value an operator can give:
// etcd's static pod carries /var/lib/etcd as a hostPath, so any other bindTo is
// a node without a working control plane. Refused before a machine is touched.
func TestCustomizationRefusesBindToOnTheRenderedMount(t *testing.T) {
	moved := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: kubernetes-data
      device: /dev/sdb1
      bindTo: /mnt/etcd
`)
	_, err := ParseCustomizations(t.Context(), []string{moved})
	require.ErrorContains(t, err, "kubernetes-data")
	require.ErrorContains(t, err, "bindTo")

	remoded := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: kubernetes-data
      device: /dev/sdb1
      mode: "0755"
`)
	_, err = ParseCustomizations(t.Context(), []string{remoded})
	require.ErrorContains(t, err, "mode",
		"etcd refuses to start on the 0755 of a fresh ext4, so this is a node that never comes up")

	// A mount of the operator's own is theirs to place: only the rendered name
	// carries values the control plane depends on.
	own := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: data
      device: /dev/sdc1
      bindTo: /mnt/data
      mode: "0755"
`)
	parsed, err := ParseCustomizations(t.Context(), []string{own})
	require.NoError(t, err)
	require.Len(t, parsed, 1)
}

func TestCustomizationRefusesWipe(t *testing.T) {
	document := nodeConfigFor("master-0", `
  storage:
    wipe: true
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "wipe",
		"defect B1: on a single-disk machine wipe:true erases the medium before anything has been copied off it, so the field must not exist here")
}

func TestCustomizationPicksTheDiskBySerial(t *testing.T) {
	document := nodeConfigFor("master-0", `
  storage:
    diskSelector:
      serial: S4EVNF0N302134
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	spec := nodeSpec{Storage: storage{
		DiskSelector: &diskSelector{Size: ">=20Gi"},
		Mounts:       etcdMounts(),
	}}
	applyCustomization(&spec, parsed[0])

	require.Equal(t, "S4EVNF0N302134", spec.Storage.DiskSelector.Serial)
	require.Empty(t, spec.Storage.DiskSelector.Size)
	require.Equal(t, etcdMounts(), spec.Storage.Mounts,
		"a diskSelector-only document must not take the etcd disk away")
}

func TestCustomizationRoutesOnlyKeepsTheRenderedInterfaces(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    routes:
    - name: storage
      networks: ["10.9.0.0/16"]
      gateway: 10.0.0.254
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	spec := nodeSpec{Network: network{
		Hostname:   "master-0",
		Interfaces: []networkInterface{{Name: "eth0", DHCP: true}},
	}}
	applyCustomization(&spec, parsed[0])

	require.Equal(t, []networkInterface{{Name: "eth0", DHCP: true}}, spec.Network.Interfaces,
		"a routes-only document must not leave the machine with no interface configuration")
	require.Equal(t, "master-0", spec.Network.Hostname)
	require.Len(t, spec.Network.Routes, 1)
}

func TestCustomizationMergesMountsByName(t *testing.T) {
	added := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: data
      device: /dev/sdc1
`)
	// The whole document an operator writes to move etcd onto a named disk: the
	// mount point and the mode are the render's, not something they repeat.
	overriding := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: kubernetes-data
      device: /dev/sdb1
`)
	parsed, err := ParseCustomizations(t.Context(), []string{added, overriding})
	require.NoError(t, err)

	spec := nodeSpec{Storage: storage{Mounts: etcdMounts()}}
	applyCustomization(&spec, parsed[0])

	require.Len(t, spec.Storage.Mounts, 2, "a mount of the operator's own must not take the etcd disk away")
	require.Equal(t, etcdMounts()[0], spec.Storage.Mounts[0])
	require.Equal(t, "data", spec.Storage.Mounts[1].Name)

	spec = nodeSpec{Storage: storage{Mounts: etcdMounts()}}
	applyCustomization(&spec, parsed[1])

	require.Len(t, spec.Storage.Mounts, 1, "a mount of the rendered name replaces it instead of joining it")
	require.Equal(t, "/dev/sdb1", spec.Storage.Mounts[0].Device)
	require.Nil(t, spec.Storage.Mounts[0].PartitionSelector)
	require.Equal(t, "/var/lib/etcd", spec.Storage.Mounts[0].BindTo,
		"naming the etcd disk must not move it off /var/lib/etcd, which the etcd static pod carries as a hostPath")
	require.Equal(t, "0700", spec.Storage.Mounts[0].Mode,
		"etcd refuses to start on the 0755 a freshly made ext4 has")
}

func TestCustomizationDNSOnlyKeepsTheRenderedInterfaces(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    dns:
      servers: ["10.0.0.53"]
      search: ["example.com"]
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	spec := nodeSpec{Network: network{
		Hostname:   "master-0",
		Interfaces: []networkInterface{{Name: "eth0", DHCP: true}},
	}}
	applyCustomization(&spec, parsed[0])

	require.Equal(t, []networkInterface{{Name: "eth0", DHCP: true}}, spec.Network.Interfaces,
		"a dns-only document must not leave the machine with no interface configuration")
	require.Equal(t, &dns{Servers: []string{"10.0.0.53"}, Search: []string{"example.com"}}, spec.Network.DNS)
}

func TestCustomizationRefusesHostname(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    hostname: whatever
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "network.hostname")
	require.ErrorContains(t, err, "node name")
}

// An address without a prefix length reaches the machine as a networkd
// "Address=" line, where it means a single host and leaves the machine off its
// own subnet. Silent there, so it is refused here.
func TestCustomizationRefusesAnAddressWithoutAPrefix(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    interfaces:
    - name: eno1
      dhcp: false
      addresses: ["192.168.0.101"]
      gateway: 192.168.0.1
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "master-0", "the node whose document is wrong")
	require.ErrorContains(t, err, "eno1", "the interface the address is on")
	require.ErrorContains(t, err, "192.168.0.101", "the value as it was written")
	require.ErrorContains(t, err, "192.168.0.101/24", "the form expected of it")
}

func TestCustomizationRefusesAnEmptyDiskSelector(t *testing.T) {
	document := nodeConfigFor("master-0", `
  storage:
    diskSelector: {}
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "diskSelector",
		"an empty selector matches every disk, config drives included, and silently widens what the render narrowed")
}

func TestCustomizationDeviceOnlyClearsTheRenderedSelector(t *testing.T) {
	document := nodeConfigFor("master-0", `
  storage:
    device: /dev/nvme0n1
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	spec := nodeSpec{Storage: storage{DiskSelector: &diskSelector{Size: ">=20Gi"}}}
	applyCustomization(&spec, parsed[0])

	require.Equal(t, "/dev/nvme0n1", spec.Storage.Device)
	require.Nil(t, spec.Storage.DiskSelector,
		"the CRD gives diskSelector priority over device, so a leftover selector would pick another disk")
}

func TestCustomizationMountsOnlyKeepTheRenderedSelector(t *testing.T) {
	document := nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: data
      device: /dev/sdc1
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	spec := nodeSpec{Storage: storage{DiskSelector: &diskSelector{Size: ">=20Gi"}}}
	applyCustomization(&spec, parsed[0])

	require.NotNil(t, spec.Storage.DiskSelector,
		"a mounts-only document says nothing about the disk the OS installs onto")
	require.Equal(t, ">=20Gi", spec.Storage.DiskSelector.Size)
}

// TestCustomizationReachesTheMasterPayload is the wiring on the payload that
// installs the cluster; TestCustomizationReachesTheJoinPayload is the other one.
func TestCustomizationReachesTheMasterPayload(t *testing.T) {
	document := nodeConfigFor("example-master-0", `
  network:
    interfaces:
    - name: eno1
      dhcp: false
      addresses: ["10.0.0.11/24"]
  storage:
    device: /dev/nvme0n1
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	globalOptions := options.NewGlobalOptions()
	payload, _, err := BuildMasterPayload(t.Context(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    testMetaConfig(t),
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
		Customization: &parsed[0],
	})
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	require.Contains(t, string(decoded), "10.0.0.11/24")
	require.Contains(t, string(decoded), "device: /dev/nvme0n1")
}

// TestCustomizationReachesTheJoinPayload is the wiring: applyCustomization is
// called from both builders, and only a rendered payload proves it.
func TestCustomizationReachesTheJoinPayload(t *testing.T) {
	document := nodeConfigFor("master-1", `
  network:
    interfaces:
    - name: eno1
      dhcp: false
      addresses: ["10.0.0.12/24"]
`)
	parsed, err := ParseCustomizations(t.Context(), []string{document})
	require.NoError(t, err)

	payload, _, err := BuildJoinPayload(t.Context(), JoinPayloadInput{
		NodeName:           "master-1",
		MetaConfig:         testMetaConfig(t),
		CACert:             "dGVzdC1jYQ==",
		BootstrapToken:     immutabletest.BootstrapToken,
		APIServerEndpoints: []string{"https://10.0.0.11:6443"},
		Customization:      &parsed[0],
	})
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	require.Contains(t, string(decoded), "10.0.0.12/24")
}

// The machine dhctl configures at one address is the machine the cluster has to
// reach afterwards. Nothing in a bare inventory-less build names an interface,
// so the push address stands and the node resolves the rest itself.
func TestThePushAddressStandsWhenNothingNamesAnInterface(t *testing.T) {
	globalOptions := options.NewGlobalOptions()
	master, nodeConfig, err := BuildMasterPayload(t.Context(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    testMetaConfig(t),
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
		PushAddress:   "10.0.0.11",
	})
	require.NoError(t, err)
	require.NotContains(t, string(nodeConfig), "cluster: true",
		"nothing about this machine is known, so no interface may be marked")

	address, err := NodeAddressAfterInstall(testMetaConfig(t), nil, nil, "10.0.0.11")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.11", address)

	require.NotContains(t, decodePayload(t, master), "nodeIP",
		"kubelet no longer takes an address from the document; the interface carries the mark instead")
}

// A mount replaces the rendered one whole, so one naming no disk used to delete
// the rendered partitionSelector: the node got a mount pointing at nothing — a
// document the NodeConfig CRD rejects (has(device) != has(partitionSelector)),
// and a first master bringing etcd up on the root filesystem.
func TestCustomizationMountsMustNameADisk(t *testing.T) {
	_, err := ParseCustomizations(t.Context(), []string{nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: kubernetes-data
      filesystem: xfs
`)})
	require.ErrorContains(t, err, "names no disk for the kubernetes-data mount")

	// A mount of the operator's own has no rendered disk to fall back on either.
	_, err = ParseCustomizations(t.Context(), []string{nodeConfigFor("master-0", `
  storage:
    mounts:
    - name: workload
      bindTo: /mnt/workload
`)})
	require.ErrorContains(t, err, "names no disk for the workload mount")
}

// The operator's mark is the one thing dhctl may not overrule: on a machine with
// two networks it is the only statement of which one the cluster is on.
func TestTheMarkedInterfaceBeatsEveryOtherRung(t *testing.T) {
	parsed, err := ParseCustomizations(t.Context(), []string{nodeConfigFor("master-1", `
  network:
    interfaces:
    - name: eno1
      dhcp: false
      addresses: ["192.168.0.59/24"]
    - name: eno2
      dhcp: false
      addresses: ["10.0.0.12/24"]
      cluster: true
`)})
	require.NoError(t, err)

	payload, _, err := BuildJoinPayload(t.Context(), JoinPayloadInput{
		NodeName:           "master-1",
		MetaConfig:         testMetaConfig(t),
		CACert:             "dGVzdC1jYQ==",
		BootstrapToken:     immutabletest.BootstrapToken,
		APIServerEndpoints: []string{"https://10.0.0.11:6443"},
		Customization:      &parsed[0],
		Inventory:          &Inventory{Interfaces: twoNICs},
		PushAddress:        "192.168.0.59",
	})
	require.NoError(t, err)

	marked := markedInterfaceOf(t, decodePayload(t, payload))
	require.Equal(t, "eno2", marked.Name)

	address, err := NodeAddressAfterInstall(testMetaConfig(t), &parsed[0], &Inventory{Interfaces: twoNICs}, "192.168.0.59")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.12", address,
		"the node answers on the interface the operator marked, not where it was pushed at")
}

func TestCustomizationRefusesAnotherKind(t *testing.T) {
	document := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master-0
spec: {}
`
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "deckhouse.io/v1/NodeGroup")
	require.ErrorContains(t, err, "internal.deckhouse.io/v1alpha1/NodeConfig")
}

// A DHCP interface never has its addresses read: renderNetwork emits Address=
// only in the static branch (images/init/src/0.1/generate.go). The document
// boots, so it is not refused — but the operator most likely meant dhcp: false.
func TestCustomizationWarnsAboutAddressesOnADHCPInterface(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    interfaces:
    - name: eno1
      dhcp: true
      addresses: ["192.168.0.101"]
    - name: eno2
      dhcp: true
`)
	var log bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&log, nil)))

	parsed, err := ParseCustomizations(ctx, []string{document})

	require.NoError(t, err, "a machine that takes DHCP boots fine whatever the addresses say")
	require.Len(t, parsed, 1)

	warnings := strings.Count(log.String(), "level=WARN")
	require.Equal(t, 1, warnings, "one warning for the offending interface and nothing for the clean one: %s", log.String())
	require.Contains(t, log.String(), "master-0", "the node whose document is odd")
	require.Contains(t, log.String(), "eno1", "the interface the addresses are on")
	require.Contains(t, log.String(), "192.168.0.101", "the value that is ignored")
	require.Contains(t, log.String(), "dhcp", "why it is ignored")
}

// twoNICs is the machine the whole ladder exists for: the network dhctl pushes
// over, and the one the cluster lives on.
var twoNICs = []InventoryInterface{
	{Name: "eno1", Addresses: []string{"192.168.0.59/24"}},
	{Name: "eno2", Addresses: []string{"10.0.0.12/24"}},
}

// markedInterfaceOf reads the interface the document marks as the cluster one,
// the way the node does.
func markedInterfaceOf(t *testing.T, document string) networkInterface {
	t.Helper()

	var parsed nodeConfig
	require.NoError(t, yaml.Unmarshal([]byte(nodeDocuments(t, document)[NodeConfigKind]), &parsed))

	var marked []networkInterface
	for _, iface := range parsed.Spec.Network.Interfaces {
		if iface.Cluster {
			marked = append(marked, iface)
		}
	}
	require.Len(t, marked, 1, "exactly one interface carries the cluster mark")
	return marked[0]
}

// nodeConfigFor prepends the header every customization carries, so a case is
// only the spec it is about.
func nodeConfigFor(nodeName, spec string) string {
	return "apiVersion: " + PayloadAPIVersion + "\nkind: " + NodeConfigKind + "\nmetadata:\n  name: " + nodeName + "\nspec:" + spec
}

// TestParseCustomizationsTakesEveryPartitionSelectorField is the CRD contract:
// the operator picks the etcd partition with what the machine's own inventory
// advertises, and every field the NodeConfig CRD offers has to reach the node.
func TestParseCustomizationsTakesEveryPartitionSelectorField(t *testing.T) {
	document := `
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  storage:
    mounts:
    - name: kubernetes-data
      partitionSelector:
        name: sdb1
        uuid: 4d3a1b2c-0000-0000-0000-000000000000
        label: kubernetes-data
        fsType: ext4
        partUUID: 8f2b1a3c-0000-0000-0000-000000000000
        partLabel: data
        size: ">=10Gi"
        blank: false
`

	customizations, err := ParseCustomizations(t.Context(), []string{document})

	require.NoError(t, err)
	require.Len(t, customizations, 1)
	selector := customizations[0].storage.Mounts[0].PartitionSelector
	require.Equal(t, &partitionSelector{
		Name: "sdb1", UUID: "4d3a1b2c-0000-0000-0000-000000000000",
		Label: "kubernetes-data", FSType: "ext4",
		PartUUID: "8f2b1a3c-0000-0000-0000-000000000000", PartLabel: "data",
		Size: ">=10Gi",
	}, selector)
}

// The CRD declares dhcp with no default, so a document naming an interface and
// nothing else asks for a NIC configured with neither DHCP= nor Address=. The
// interfaces replace the rendered ones wholesale, so the machine never joins.
func TestCustomizationRefusesAnInterfaceWithNeitherDHCPNorAddress(t *testing.T) {
	document := nodeConfigFor("master-0", `
  network:
    interfaces:
    - name: eno1
`)
	_, err := ParseCustomizations(t.Context(), []string{document})

	require.ErrorContains(t, err, "master-0", "the node whose document is wrong")
	require.ErrorContains(t, err, "eno1", "the interface that would come up unconfigured")
	require.ErrorContains(t, err, "address", "one way out")
	require.ErrorContains(t, err, "dhcp: true", "the other")
}
