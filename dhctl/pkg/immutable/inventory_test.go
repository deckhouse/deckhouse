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
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// The invariant, not the mechanics: what dhctl hands a machine is a stream of
// documents, and a stream carries no disk and no interface of its own. A checker
// that answers "nothing wrong" to it calls every machine healthy — as one live
// stand was, for every machine, until this was caught.
func TestCheckDocumentAgainstInventoryRefusesTheWholeStream(t *testing.T) {
	globalOptions := options.NewGlobalOptions()
	payload, _, err := BuildMasterPayload(t.Context(), MasterPayloadInput{
		NodeName:      "example-master-0",
		MetaConfig:    testMetaConfig(t),
		StateCache:    cache.NewTestCache(),
		CandiDir:      testCandiDir(t),
		GlobalOptions: &globalOptions,
	})
	if err != nil {
		t.Fatalf("build the master payload: %v", err)
	}
	stream, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode the master payload: %v", err)
	}

	inv := &Inventory{Disks: []InventoryDisk{
		{Name: "sda", Size: 32212254720},
		{Name: "sdb", Size: 32212254720},
	}}

	if err := CheckDocumentAgainstInventory(t.Context(), stream, inv); err == nil {
		t.Fatal("the whole stream was accepted as a NodeConfig, so every machine would pass")
	}
}

// A single document of the wrong kind parses into an empty spec, where every
// check passes. The refusal has to say which document was wanted.
func TestCheckDocumentAgainstInventoryRefusesAnotherKind(t *testing.T) {
	document := []byte("apiVersion: " + PayloadAPIVersion + "\nkind: ControlPlaneConfig\nspec:\n  bootstrap: true\n")

	err := CheckDocumentAgainstInventory(t.Context(), document, &Inventory{
		Disks: []InventoryDisk{{Name: "sda", Size: 32212254720}},
	})
	if err == nil {
		t.Fatal("a document that is not a NodeConfig must be refused, not silently approved")
	}
	if !strings.Contains(err.Error(), NodeConfigKind) {
		t.Fatalf("the refusal must name the document it wanted, got %v", err)
	}
}

func TestFetchInventoryTreatsMissingEndpointAsUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	inv, err := FetchInventory(t.Context(), strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("an image without the endpoint must not fail the bootstrap: %v", err)
	}
	if inv != nil {
		t.Fatalf("want nil inventory, got %+v", inv)
	}
}

func TestFetchInventoryReadsTheMachine(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = io.WriteString(w, `{"disks":[{"name":"sda","size":32212254720,"state":"blank",
			"byId":["scsi-0QEMU_QEMU_HARDDISK_89bf"],"partitions":[{"name":"sda1","size":536870912}]}],
			"interfaces":[{"name":"eth0","mac":"f2:4e:c6:60:03:72"}]}`)
	}))
	defer srv.Close()

	inv, err := FetchInventory(t.Context(), strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	// /inventory answers an operator with the NodeConfig shape; a program reads
	// the representation meant for one, or it decodes comments.
	if path != "/inventory.json" {
		t.Fatalf("dhctl must read the machine-readable inventory, asked for %q", path)
	}
	if len(inv.Disks) != 1 || inv.Disks[0].Size != 32212254720 || inv.Disks[0].State != "blank" {
		t.Fatalf("disks did not survive the decode: %+v", inv.Disks)
	}
	if len(inv.Disks[0].Partitions) != 1 || inv.Disks[0].Partitions[0].Name != "sda1" {
		t.Fatalf("partitions did not survive the decode: %+v", inv.Disks[0].Partitions)
	}
	if len(inv.Interfaces) != 1 || inv.Interfaces[0].MAC != "f2:4e:c6:60:03:72" {
		t.Fatalf("interfaces did not survive the decode: %+v", inv.Interfaces)
	}
}

func TestFetchInventoryReportsARefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "collect the inventory: no /sys/block")
	}))
	defer srv.Close()

	_, err := FetchInventory(t.Context(), strings.TrimPrefix(srv.URL, "http://"))
	if err == nil || !strings.Contains(err.Error(), "no /sys/block") {
		t.Fatalf("a server that answers with a reason must have it quoted, got %v", err)
	}
}

func TestCheckDocumentAgainstInventory(t *testing.T) {
	inv := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, ByPath: "pci-0000:0d:00.0-scsi-0:0:0:0"},
			{Name: "sdb", Size: 32212254720, ByPath: "pci-0000:0d:00.0-scsi-0:0:0:1"},
			{Name: "sdc", Size: 10737418240, ByPath: "pci-0000:0d:00.0-scsi-0:0:0:2"},
		},
		Interfaces: []InventoryInterface{{Name: "eth0", MAC: "f2:4e:c6:60:03:72"}},
	}

	cases := []struct {
		name     string
		document string
		want     string
	}{
		{
			name: "size shared by two disks",
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: "=30Gi"
`),
			want: "matches 2 disks",
		},
		{
			name: "device that does not exist",
			document: nodeConfigWithSpec(`
  storage:
    device: /dev/disk/by-path/pci-0000:0d:00.0-scsi-0:0:0:9
`),
			want: "no disk",
		},
		{
			name: "an address hung on an interface that does not exist",
			document: nodeConfigWithSpec(`
  network:
    interfaces:
    - name: eth1
      addresses: ["10.0.0.11/24"]
`),
			want: "eth1",
		},
		{
			// Most hardware names its NIC enp3s0 or eno1, and the rendered default
			// says eth0 on DHCP: refusing that name would refuse the machine itself.
			name: "a name the machine lacks, on DHCP, is the installer's own guess",
			document: nodeConfigWithSpec(`
  network:
    interfaces:
    - name: eth9
      dhcp: true
`),
			want: "",
		},
		{
			name: "a mount device that names nothing",
			document: nodeConfigWithSpec(`
  storage:
    mounts:
    - name: kubernetes-data
      device: /dev/sdz1
`),
			want: "names no device of this machine",
		},
		{
			name: "unique size passes",
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: "=10Gi"
`),
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckDocumentAgainstInventory(t.Context(), []byte(c.document), inv)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error mentioning %q, got %v", c.want, err)
			}
		})
	}
}

// The operator who reads this error cannot log into the machine: the refusal is
// the only place they learn what the machine actually has, so it carries every
// fact they could write a selector on.
func TestRefusalNamesTheMachineAndWhatItHas(t *testing.T) {
	inv := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank", ByPath: "pci-0000:0d:00.0-scsi-0:0:0:0",
				Model: "QEMU HARDDISK", Serial: "89bf9603"},
			{Name: "sdb", Size: 32212254720, State: "blank", ByPath: "pci-0000:0d:00.0-scsi-0:0:0:1",
				Model: "QEMU HARDDISK", Serial: "c41d77a2"},
		},
	}
	document := nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20Gi"
`)

	err := CheckDocumentAgainstInventory(t.Context(), []byte(document), inv)
	if err == nil {
		t.Fatal("the incident document must be refused")
	}
	for _, want := range []string{
		"spec.storage.diskSelector",
		"sda", "sdb", "32212254720",
		// Everything the operator could select on, so they pick instead of being
		// handed one answer the machine chose for them.
		"/dev/disk/by-path/pci-0000:0d:00.0-scsi-0:0:0:1",
		"serial c41d77a2", "model QEMU HARDDISK",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal must name %q, got: %v", want, err)
		}
	}
}

func TestCheckDocumentAcceptsWhatTheMachineWillMatch(t *testing.T) {
	twoDisks := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank", ByPath: "pci-0000:0d:00.0-scsi-0:0:0:0",
				ByID: []string{"scsi-0QEMU_QEMU_HARDDISK_89bf"}},
			{Name: "sdb", Size: 10737418240, State: "blank", ByPath: "pci-0000:0d:00.0-scsi-0:0:0:1"},
		},
		Interfaces: []InventoryInterface{{Name: "eth0", MAC: "f2:4e:c6:60:03:72"}},
	}
	oneDisk := &Inventory{
		Disks:      []InventoryDisk{{Name: "sda", Size: 32212254720, State: "blank"}},
		Interfaces: []InventoryInterface{{Name: "eth0"}},
	}
	usedDisk := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank"},
			{Name: "sdb", Size: 42949672960, State: "system-layout", Partitions: []InventoryPartition{
				{Name: "sdb1", Size: 536870912, Label: "BOOT"},
				{Name: "sdb2", Size: 10485760, Label: "CONFIG"},
				{Name: "sdb3", Size: 21474836480, Label: "DATA"},
				{Name: "sdb4", Size: 21474836480, Label: "backups"},
			}},
		},
		Interfaces: []InventoryInterface{{Name: "eth0"}},
	}

	cases := []struct {
		name      string
		inventory *Inventory
		document  string
	}{
		{
			name:      "a bare size means at least this much, as on the node",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20Gi"
`),
		},
		{
			name:      "a device named by its by-id link",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    device: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_89bf
`),
		},
		{
			name:      "the rendered etcd mount on a machine with a spare disk",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20Gi"
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
        blank: true
      bindTo: /var/lib/etcd
`),
		},
		{
			// etcdMounts (nodeconfig.go) is rendered for every control-plane node,
			// and the node skips a mount that matches nothing.
			name:      "the rendered etcd mount on a machine with one disk",
			inventory: oneDisk,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20Gi"
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
        blank: true
      bindTo: /var/lib/etcd
`),
		},
		{
			// The machine reads the serial off its by-id link when sysfs has none,
			// and the inventory publishes an empty one there.
			name:      "a serial the machine only knows from its by-id link",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      serial: 89bf
`),
		},
		{
			name:      "a bus path in the spelling of /dev/disk/by-path",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      busPath: pci-0000:0d:00.0-scsi-0:0:0:1
`),
		},
		{
			name:      "a mount that may not reach the OS partitions",
			inventory: usedDisk,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      name: sda
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
`),
		},
		{
			name:      "a static address kubelet is told to register with",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  network:
    interfaces:
    - name: eth0
      dhcp: false
      addresses: ["192.168.0.101/24"]
  kubelet:
    nodeIP: 192.168.0.101
`),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := CheckDocumentAgainstInventory(t.Context(), []byte(c.document), c.inventory); err != nil {
				t.Fatalf("the machine satisfies this document, got %v", err)
			}
		})
	}
}

func TestCheckDocumentRefusesWhatTheMachineCannotSatisfy(t *testing.T) {
	threeBlank := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank"},
			{Name: "sdb", Size: 10737418240, State: "blank"},
			{Name: "sdc", Size: 10737418240, State: "blank"},
		},
		Interfaces: []InventoryInterface{{Name: "eth0"}},
	}

	cases := []struct {
		name      string
		inventory *Inventory
		document  string
		want      string
	}{
		{
			name:      "two spare disks fit the etcd mount",
			inventory: threeBlank,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20Gi"
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
        blank: true
`),
			want: "matches 2 devices",
		},
		{
			name:      "a size nothing on the machine is that big",
			inventory: threeBlank,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=100Gi"
`),
			want: "matches no disk",
		},
		{
			name:      "a size expression the node cannot parse",
			inventory: threeBlank,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=30 gigs"
`),
			want: "unknown unit",
		},
		{
			name:      "kubelet registers with an address no interface is given",
			inventory: threeBlank,
			document: nodeConfigWithSpec(`
  network:
    interfaces:
    - name: eth0
      dhcp: false
      addresses: ["192.168.0.101/24"]
  kubelet:
    nodeIP: 192.168.0.5
`),
			want: "192.168.0.5",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckDocumentAgainstInventory(t.Context(), []byte(c.document), c.inventory)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error mentioning %q, got %v", c.want, err)
			}
		})
	}
}

// The node matches the same expressions with matchSize/parseSize of
// images/init/src/0.1/disk.go; a disagreement refuses a document that would
// have worked, or passes one that will not.
func TestSizeGrammarAgreesWithTheNode(t *testing.T) {
	const thirtyGiB = 30 << 30

	cases := []struct {
		expr   string
		actual uint64
		want   bool
	}{
		{"10Gi", thirtyGiB, true},                  // no operator means ">="
		{"40Gi", thirtyGiB, false},                 //
		{"=30Gi", thirtyGiB, true},                 //
		{"=30Gi", thirtyGiB + thirtyGiB/200, true}, // "=" tolerates 1%
		{"=30Gi", thirtyGiB + thirtyGiB/50, false}, //
		{"30G", 30_000_000_000, true},              // a bare G is 10^9, not 2^30
		{">30Gi", thirtyGiB, false},                //
		{"<=30Gi", thirtyGiB, true},                //
		{" >= 30 Gi ", thirtyGiB, true},            // spaces are trimmed
	}
	for _, c := range cases {
		got, err := matchSize(c.expr, c.actual)
		if err != nil {
			t.Fatalf("matchSize(%q, %d): %v", c.expr, c.actual, err)
		}
		if got != c.want {
			t.Fatalf("matchSize(%q, %d) = %v, want %v", c.expr, c.actual, got, c.want)
		}
	}
}

func TestCheckDocumentAgainstNoInventoryIsNoCheckAtAll(t *testing.T) {
	document := nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: "=30Gi"
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
        blank: true
  network:
    interfaces:
    - name: eth0
      dhcp: true
  kubelet:
    nodeIP: 192.168.0.5
`)

	if err := CheckDocumentAgainstInventory(t.Context(), []byte(document), nil); err != nil {
		t.Fatalf("a machine that serves no inventory leaves nothing to check against, got %v", err)
	}
}

// Two selectors, two consumers, two grammars: the initramfs resolves
// diskSelector (images/init/src/0.1/disk.go) and the agent resolves
// partitionSelector (nodelet/internal/config/size.go, Kubernetes quantities).
func TestSizeGrammarFollowsItsConsumer(t *testing.T) {
	twoDisks := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank"},
			{Name: "sdb", Size: 10737418240, State: "blank"},
		},
	}
	threeDisks := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank"},
			{Name: "sdb", Size: 10737418240, State: "blank"},
			{Name: "sdc", Size: 32212254720, State: "blank"},
		},
	}

	cases := []struct {
		name      string
		inventory *Inventory
		document  string
		want      string
	}{
		{
			name:      "a quantity suffix the initramfs does not know is refused for the OS disk",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: 30k
`),
			want: "unknown unit",
		},
		{
			name:      "the same suffix is a plain quantity for a mount",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    device: /dev/sda
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 30k
        blank: true
`),
			want: "",
		},
		{
			name:      "a suffix the initramfs knows is no quantity for a mount",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    device: /dev/sda
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10GB
        blank: true
`),
			want: "is not a size",
		},
		{
			name:      "the same suffix is fine for the OS disk",
			inventory: twoDisks,
			document: nodeConfigWithSpec(`
  storage:
    diskSelector:
      size: ">=20GB"
`),
			want: "",
		},
		{
			name:      "a bare size for a mount means at least this much, as on the node",
			inventory: threeDisks,
			document: nodeConfigWithSpec(`
  storage:
    device: /dev/sda
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
        blank: true
`),
			want: "matches 2 devices",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckDocumentAgainstInventory(t.Context(), []byte(c.document), c.inventory)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error mentioning %q, got %v", c.want, err)
			}
		})
	}
}

// The node looks its OS labels up case-sensitively (osPartitionLabels,
// nodelet/internal/controllers/mounts/partitions.go), so a partition labelled
// "data" is a candidate there and has to be counted as one here.
func TestOSPartitionLabelsAreCaseSensitiveLikeTheNode(t *testing.T) {
	inv := &Inventory{
		Disks: []InventoryDisk{
			{Name: "sda", Size: 32212254720, State: "blank"},
			{Name: "sdb", Size: 42949672960, State: "formatted", Partitions: []InventoryPartition{
				{Name: "sdb1", Size: 21474836480, Label: "data"},
				{Name: "sdb2", Size: 21474836480, Label: "spare"},
			}},
		},
	}
	document := nodeConfigWithSpec(`
  storage:
    device: /dev/sda
    mounts:
    - name: kubernetes-data
      partitionSelector:
        size: 10Gi
`)

	err := CheckDocumentAgainstInventory(t.Context(), []byte(document), inv)
	if err == nil || !strings.Contains(err.Error(), "matches 2 devices") {
		t.Fatalf("both partitions are candidates on the node, got %v", err)
	}
}

// nodeConfigWithSpec prepends the header every document under test carries, so
// a case is only the spec it is about.
func nodeConfigWithSpec(spec string) string {
	return "apiVersion: " + PayloadAPIVersion + "\nkind: " + NodeConfigKind + "\nspec:" + spec
}
