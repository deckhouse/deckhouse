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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"path"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// inventoryPath is where a machine waiting for its configuration answers with
// its hardware, and inventoryAccept is the representation this client reads.
// One path serves three: the machine defaults to the NodeConfig shape an
// operator merges by hand, and hands JSON to whoever asks for it. Mirrors
// images/init/src/0.1/acquire.go of the initramfs repository, which registers
// the route.
const (
	inventoryPath   = "/inventory"
	inventoryAccept = "application/json"
)

// Inventory is what a machine says about itself before anything is installed.
// The shape is the wire contract: Inventory in images/init/src/0.1/inventory.go
// of the initramfs repository.
type Inventory struct {
	Disks      []InventoryDisk      `json:"disks"`
	Interfaces []InventoryInterface `json:"interfaces"`
}

type InventoryDisk struct {
	Name       string   `json:"name"`
	Size       uint64   `json:"size"`
	Model      string   `json:"model"`
	Vendor     string   `json:"vendor"`
	Serial     string   `json:"serial"`
	WWID       string   `json:"wwid"`
	Rotational bool     `json:"rotational"`
	Transport  string   `json:"transport"`
	ByPath     string   `json:"byPath"`
	ByID       []string `json:"byId"`
	BusPath    string   `json:"busPath"`
	// State is blank, formatted or system-layout — what the size cannot say.
	State      string               `json:"state"`
	Partitions []InventoryPartition `json:"partitions"`
}

type InventoryPartition struct {
	Name   string `json:"name"`
	Size   uint64 `json:"size"`
	FSType string `json:"fsType"`
	Label  string `json:"label"`
}

type InventoryInterface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	Link      string   `json:"link"`
	Addresses []string `json:"addresses"`
	Gateway   string   `json:"gateway"`
	Source    string   `json:"source"`
}

// ErrInventoryUnusable marks an answer that came back unusable: a refusal, a body
// that does not parse. A machine that answers nothing at all is not one of these
// — it is still booting, and only this sentinel tells the caller which happened.
var ErrInventoryUnusable = errors.New("the machine answered no usable inventory")

// FetchInventory reads what the machine says about its own hardware. A machine
// whose image predates the endpoint answers 404, and that is nil, nil: an old
// image is a check nobody can run, not a bootstrap to fail.
func FetchInventory(ctx context.Context, address string) (*Inventory, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+inventoryPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build the inventory request for %s: %w", address, err)
	}
	request.Header.Set("Accept", inventoryAccept)

	client := &http.Client{Timeout: pushTimeout}
	defer client.CloseIdleConnections()

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read the inventory of %s: %w", address, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, err := io.ReadAll(io.LimitReader(response.Body, maxPushErrorBody))
		if err != nil {
			return nil, fmt.Errorf("%w: read the inventory of %s: %s: read the refusal: %w", ErrInventoryUnusable, address, response.Status, err)
		}
		return nil, fmt.Errorf("%w: read the inventory of %s: %s: %s", ErrInventoryUnusable, address, response.Status, bytes.TrimSpace(body))
	}

	var inventory Inventory
	if err := json.NewDecoder(response.Body).Decode(&inventory); err != nil {
		return nil, fmt.Errorf("%w: read the inventory of %s: %w", ErrInventoryUnusable, address, err)
	}
	return &inventory, nil
}

// CheckDocumentAgainstInventory refuses a NodeConfig this machine cannot satisfy,
// while nothing is installed yet. It takes that document itself, never the
// cloud-init carrying it: a wrapper names no disk, no interface, and would pass.
func CheckDocumentAgainstInventory(ctx context.Context, nodeConfigDocument []byte, inventory *Inventory) error {
	// An image too old to serve an inventory answers 404, which FetchInventory
	// reports as no inventory: there is then nothing to check the document against.
	if inventory == nil {
		return nil
	}

	var parsed nodeConfig
	if err := yaml.Unmarshal(nodeConfigDocument, &parsed); err != nil {
		return fmt.Errorf("read the document to check it against the machine: %w", err)
	}
	// A document of any other shape parses into an empty spec, and every check
	// below then passes on every machine. Refused rather than warned: the caller
	// is dhctl itself, and the wrong document here is a bug in it.
	if parsed.APIVersion != payloadAPIVersion || parsed.Kind != nodeConfigKind {
		return fmt.Errorf("the document to check against the machine is %q/%q, not %s/%s: it says nothing about disks or interfaces",
			parsed.APIVersion, parsed.Kind, payloadAPIVersion, nodeConfigKind)
	}
	spec := parsed.Spec

	problems := []error{checkSystemDisk(spec.Storage, inventory)}

	systemDisk := resolveSystemDisk(spec.Storage, inventory)
	for i, m := range spec.Storage.Mounts {
		problems = append(problems, checkMount(i, m, inventory, systemDisk))
	}
	for i, iface := range spec.Network.Interfaces {
		problems = append(problems, checkInterface(ctx, i, iface, inventory))
	}
	problems = append(problems, checkNodeIP(spec))

	return errors.Join(problems...)
}

// checkSystemDisk answers whether the machine has the one disk the OS is to be
// installed on. The node reads diskSelector first and never looks at device
// when it is set (selectDisk, images/init/src/0.1/disk.go), so neither does this.
func checkSystemDisk(st storage, inventory *Inventory) error {
	if st.DiskSelector != nil {
		return checkDiskSelector(st.DiskSelector, inventory)
	}
	if st.Device == "" {
		return nil
	}
	if diskOfDevicePath(st.Device, inventory.Disks) != "" {
		return nil
	}
	return fmt.Errorf("spec.storage.device %s names no disk of this machine, which has: %s",
		st.Device, describeDisks(inventory.Disks))
}

func checkDiskSelector(selector *diskSelector, inventory *Inventory) error {
	matched, err := matchDisks(selector, inventory.Disks)
	if err != nil {
		return fmt.Errorf("spec.storage.diskSelector: %w", err)
	}

	switch len(matched) {
	case 1:
		return nil
	case 0:
		return fmt.Errorf("spec.storage.diskSelector matches no disk of this machine, which has: %s",
			describeDisks(inventory.Disks))
	default:
		return fmt.Errorf("spec.storage.diskSelector matches %d disks of this machine and only one can hold "+
			"the system: %s. Name a single disk instead of an attribute several share",
			len(matched), describeDisks(matched))
	}
}

// checkMount refuses a selector matching more than one device, as the node does.
// Zero is not a refusal: the node skips a mount that matches nothing, which is
// how a single-disk master keeps etcd on the data partition.
func checkMount(index int, m mount, inventory *Inventory, systemDisk string) error {
	if m.PartitionSelector == nil {
		return checkMountDevice(index, m, inventory)
	}
	field := fmt.Sprintf("spec.storage.mounts[%d] (%s).partitionSelector", index, m.Name)

	matched, err := mountCandidates(m.PartitionSelector, inventory, systemDisk)
	if err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if len(matched) <= 1 {
		return nil
	}
	return fmt.Errorf("%s matches %d devices (%s) and the node refuses to choose between them: "+
		"narrow the selector, or name the device with spec.storage.mounts[%d].device",
		field, len(matched), strings.Join(matched, ", "), index)
}

// checkMountDevice refuses a device that names nothing here. A mount may name a
// partition as well as a whole disk, which the system disk may not.
func checkMountDevice(index int, m mount, inventory *Inventory) error {
	if m.Device == "" || mountDeviceExists(m.Device, inventory.Disks) {
		return nil
	}
	return fmt.Errorf("spec.storage.mounts[%d] (%s).device %s names no device of this machine, which has: %s",
		index, m.Name, m.Device, describeDisks(inventory.Disks))
}

func mountDeviceExists(devicePath string, disks []InventoryDisk) bool {
	if diskOfDevicePath(devicePath, disks) != "" {
		return true
	}
	for _, disk := range disks {
		for _, part := range disk.Partitions {
			if devicePath == "/dev/"+part.Name {
				return true
			}
		}
	}
	return false
}

// mountCandidates lists what the selector finds on this machine. Mirrors
// selectPartition in internal/controllers/mounts/partitions.go of the nodelet
// repository: whole disks only when blank, never the OS partitions.
func mountCandidates(selector *partitionSelector, inventory *Inventory, systemDisk string) ([]string, error) {
	var matched []string

	for _, disk := range inventory.Disks {
		// By the time the node resolves the mounts, this disk carries the OS: it
		// is neither blank any more nor holds a partition a selector may reach.
		if disk.Name == systemDisk {
			continue
		}
		if selector.Blank && disk.State == "blank" {
			ok, err := partitionSizeMatches(selector.Size, disk.Size)
			if err != nil {
				return nil, err
			}
			if ok {
				matched = append(matched, "/dev/"+disk.Name)
			}
		}
		for _, part := range disk.Partitions {
			if isOSPartitionLabel(part.Label) {
				continue
			}
			ok, err := partitionSizeMatches(selector.Size, part.Size)
			if err != nil {
				return nil, err
			}
			if ok {
				matched = append(matched, "/dev/"+part.Name)
			}
		}
	}

	return matched, nil
}

// isOSPartitionLabel reports the labels of the layout the OS installs itself
// into, which no mount may ever select. Mirrors osPartitionLabels of
// nodelet/internal/controllers/mounts/partitions.go, case-sensitively as it is
// looked up there: a partition labelled "data" is a candidate on the node.
func isOSPartitionLabel(label string) bool {
	return slices.Contains([]string{"BOOT", "CONFIG", "DATA"}, label)
}

// resolveSystemDisk names the disk the OS will take, or "" when the document
// does not resolve to exactly one — an ambiguity checkSystemDisk already reports.
func resolveSystemDisk(st storage, inventory *Inventory) string {
	if st.DiskSelector == nil {
		return diskOfDevicePath(st.Device, inventory.Disks)
	}
	matched, err := matchDisks(st.DiskSelector, inventory.Disks)
	if err != nil || len(matched) != 1 {
		return ""
	}
	return matched[0].Name
}

// diskOfDevicePath resolves a /dev path against the inventory, or "" when no
// disk answers to it.
func diskOfDevicePath(devicePath string, disks []InventoryDisk) string {
	if devicePath == "" {
		return ""
	}
	for _, disk := range disks {
		if slices.Contains(devicePathsOf(disk), devicePath) {
			return disk.Name
		}
	}
	return ""
}

// devicePathsOf lists every path that names this disk: the kernel name and the
// stable links udev makes for it.
func devicePathsOf(disk InventoryDisk) []string {
	paths := []string{"/dev/" + disk.Name}
	if disk.ByPath != "" {
		paths = append(paths, "/dev/disk/by-path/"+disk.ByPath)
	}
	for _, id := range disk.ByID {
		paths = append(paths, "/dev/disk/by-id/"+id)
	}
	return paths
}

// checkInterface refuses a name the machine does not have only where addresses
// hang on it. The rendered default is eth0 on DHCP, a guess no operator wrote,
// and the node brings DHCP up on the NIC it finds — enp3s0 on most hardware.
func checkInterface(ctx context.Context, index int, iface networkInterface, inventory *Inventory) error {
	if slices.ContainsFunc(inventory.Interfaces, func(i InventoryInterface) bool { return i.Name == iface.Name }) {
		return nil
	}
	if iface.DHCP || len(iface.Addresses) == 0 {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"spec.network.interfaces[%d].name %q names no interface of this machine, which has: %s; it carries no static address, so the node configures DHCP on what it finds",
			index, iface.Name, describeInterfaces(inventory.Interfaces)))
		return nil
	}
	return fmt.Errorf("spec.network.interfaces[%d].name %q names no interface of this machine, which has: %s. "+
		"The addresses it carries would reach nothing: name an interface the machine has",
		index, iface.Name, describeInterfaces(inventory.Interfaces))
}

// checkNodeIP answers whether kubelet is told to register with an address the
// document itself gives the machine. A machine left on DHCP is not checked: the
// lease it will get is not this document's to promise.
func checkNodeIP(spec nodeSpec) error {
	if spec.Kubelet.NodeIP == "" {
		return nil
	}

	var declared []string
	for _, iface := range spec.Network.Interfaces {
		if iface.DHCP {
			continue
		}
		declared = append(declared, iface.Addresses...)
	}
	if len(declared) == 0 {
		return nil
	}
	if slices.ContainsFunc(declared, func(addr string) bool { return addressIs(addr, spec.Kubelet.NodeIP) }) {
		return nil
	}

	return fmt.Errorf("spec.kubelet.nodeIP %s is none of the addresses this document gives the machine (%s): "+
		"kubelet would register the node under an address no interface holds",
		spec.Kubelet.NodeIP, strings.Join(declared, ", "))
}

// addressIs reports whether a declared address, written as a CIDR or as a bare
// address, is the given one.
func addressIs(declared, address string) bool {
	want, err := netip.ParseAddr(address)
	if err != nil {
		return declared == address
	}
	if prefix, err := netip.ParsePrefix(declared); err == nil {
		return prefix.Addr() == want
	}
	got, err := netip.ParseAddr(declared)
	return err == nil && got == want
}

// matchDisks lists the disks the selector describes. Mirrors matchDisk and
// matchProperties in images/init/src/0.1/disk.go of the initramfs repository:
// the machine matches these fields itself and the two must not disagree.
func matchDisks(selector *diskSelector, disks []InventoryDisk) ([]InventoryDisk, error) {
	var matched []InventoryDisk
	for _, disk := range disks {
		ok, err := matchDisk(selector, disk)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, disk)
		}
	}
	return matched, nil
}

func matchDisk(selector *diskSelector, disk InventoryDisk) (bool, error) {
	if !globMatch(selector.Name, disk.Name) || !globMatch(selector.Model, disk.Model) {
		return false, nil
	}
	if !globMatch(selector.Serial, serialOf(disk)) || !globMatch(selector.WWID, wwidOf(disk)) {
		return false, nil
	}
	// The machine reports the bus path twice — as the kernel spells it and as
	// udev does — and matches a pattern written either way.
	if selector.BusPath != "" && !globMatch(selector.BusPath, disk.BusPath) {
		if !globMatch(selector.BusPath, disk.ByPath) {
			return false, nil
		}
	}
	if selector.Rotational != nil && *selector.Rotational != disk.Rotational {
		return false, nil
	}
	if selector.Type != "" && !matchType(selector.Type, disk) {
		return false, nil
	}
	return diskSizeMatches(selector.Size, disk.Size)
}

// serialOf and wwidOf recover what the machine's own matcher sees where sysfs
// publishes neither: mirrors udevSerial and udevWWID in
// images/init/src/0.1/disk.go, which read them off the by-id links.
func serialOf(disk InventoryDisk) string {
	if disk.Serial != "" || len(disk.ByID) == 0 {
		return disk.Serial
	}
	id := disk.ByID[0]
	if last := strings.LastIndex(id, "_"); last >= 0 {
		return id[last+1:]
	}
	return ""
}

func wwidOf(disk InventoryDisk) string {
	if disk.WWID != "" {
		return disk.WWID
	}
	for _, id := range disk.ByID {
		if wwn, found := strings.CutPrefix(id, "wwn-"); found {
			return wwn
		}
	}
	return ""
}

// matchType classifies the disk the way the machine does. Mirrors matchType in
// images/init/src/0.1/disk.go of the initramfs repository.
func matchType(diskType string, disk InventoryDisk) bool {
	switch strings.ToLower(diskType) {
	case "nvme":
		return disk.Transport == "nvme" || strings.HasPrefix(disk.Name, "nvme")
	case "ssd":
		return !disk.Rotational
	case "hdd":
		return disk.Rotational
	case "sd":
		return strings.HasPrefix(disk.Name, "sd")
	default:
		return false
	}
}

// globMatch reports whether the pattern constrains s. An empty pattern is no
// constraint, and * does not cross a path separator.
func globMatch(pattern, s string) bool {
	if pattern == "" {
		return true
	}
	ok, err := path.Match(pattern, s)
	return err == nil && ok
}

// diskSizeMatches and partitionSizeMatches are two grammars on purpose, because
// two different programs read them: the initramfs resolves spec.storage
// diskSelector, the node's agent resolves the mounts. Do not unify them.
func diskSizeMatches(expression string, size uint64) (bool, error) {
	if expression == "" {
		return true, nil
	}
	return matchSize(expression, size)
}

func partitionSizeMatches(expression string, size uint64) (bool, error) {
	if expression == "" {
		return true, nil
	}
	return matchPartitionSize(expression, size)
}

// matchPartitionSize evaluates the size of a partitionSelector as Kubernetes
// quantity grammar. Mirrors MatchSize and ParseSizeExpression in
// nodelet/internal/config/size.go, which is what resolves this selector.
func matchPartitionSize(expression string, actual uint64) (bool, error) {
	operator := ">="
	rest := strings.TrimSpace(expression)
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if after, found := strings.CutPrefix(rest, candidate); found {
			operator, rest = candidate, strings.TrimSpace(after)
			break
		}
	}
	if rest == "" {
		return false, errors.New("no size given")
	}

	quantity, err := resource.ParseQuantity(rest)
	if err != nil {
		return false, fmt.Errorf("%q is not a size: %w", rest, err)
	}
	if quantity.Sign() <= 0 {
		return false, fmt.Errorf("%q must be positive", rest)
	}

	want, size := quantity.Value(), int64(actual)
	switch operator {
	case ">=":
		return size >= want, nil
	case "<=":
		return size <= want, nil
	case ">":
		return size > want, nil
	case "<":
		return size < want, nil
	}
	// "=" allows one percent, as on the node: a partition rarely reports the
	// round number somebody wrote.
	return (max(size, want)-min(size, want))*100 <= want, nil
}

// matchSize evaluates a size expression like ">=100Gi" or "512Gi" against a size
// in bytes. Mirrors matchSize in images/init/src/0.1/disk.go of the initramfs
// repository — the machine matches its own disks with that one.
func matchSize(expression string, actual uint64) (bool, error) {
	operator := ">="
	rest := strings.TrimSpace(expression)
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if after, found := strings.CutPrefix(rest, candidate); found {
			operator, rest = candidate, strings.TrimSpace(after)
			break
		}
	}

	want, err := parseSize(rest)
	if err != nil {
		return false, err
	}

	switch operator {
	case ">=":
		return actual >= want, nil
	case "<=":
		return actual <= want, nil
	case ">":
		return actual > want, nil
	case "<":
		return actual < want, nil
	}
	// "=" is exact within 1%: a disk rarely reports an exact round size.
	distance := max(actual, want) - min(actual, want)
	return distance*100 <= want, nil
}

// parseSize reads a size with an optional IEC (Ki/Mi/Gi/Ti) or SI (K/M/G/T)
// suffix. Mirrors parseSize in images/init/src/0.1/disk.go of the initramfs
// repository, units included.
func parseSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	digits := 0
	for digits < len(s) && (s[digits] == '.' || (s[digits] >= '0' && s[digits] <= '9')) {
		digits++
	}
	if digits == 0 {
		return 0, fmt.Errorf("no number in %q", s)
	}
	number, err := strconv.ParseFloat(s[:digits], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.TrimSpace(s[digits:])
	multiplier, known := map[string]float64{
		"": 1, "B": 1,
		"K": 1e3, "KB": 1e3, "Ki": 1 << 10, "KiB": 1 << 10,
		"M": 1e6, "MB": 1e6, "Mi": 1 << 20, "MiB": 1 << 20,
		"G": 1e9, "GB": 1e9, "Gi": 1 << 30, "GiB": 1 << 30,
		"T": 1e12, "TB": 1e12, "Ti": 1 << 40, "TiB": 1 << 40,
	}[unit]
	if !known {
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
	return uint64(number * multiplier), nil
}

func describeDisks(disks []InventoryDisk) string {
	if len(disks) == 0 {
		return "no disks at all"
	}
	described := make([]string, 0, len(disks))
	for _, disk := range disks {
		described = append(described, describeDisk(disk))
	}
	return strings.Join(described, "; ")
}

func describeDisk(disk InventoryDisk) string {
	facts := []string{strconv.FormatUint(disk.Size, 10) + " bytes"}
	if disk.State != "" {
		facts = append(facts, disk.State)
	}
	if disk.ByPath != "" {
		facts = append(facts, "/dev/disk/by-path/"+disk.ByPath)
	}
	if disk.BusPath != "" {
		facts = append(facts, "busPath "+disk.BusPath)
	}
	if disk.Model != "" {
		facts = append(facts, "model "+disk.Model)
	}
	if disk.Serial != "" {
		facts = append(facts, "serial "+disk.Serial)
	}
	if disk.WWID != "" {
		facts = append(facts, "wwid "+disk.WWID)
	}
	return disk.Name + " (" + strings.Join(facts, ", ") + ")"
}

func describeInterfaces(interfaces []InventoryInterface) string {
	if len(interfaces) == 0 {
		return "no interfaces at all"
	}
	described := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		facts := []string{iface.MAC, iface.Link}
		facts = append(facts, iface.Addresses...)
		facts = slices.DeleteFunc(facts, func(fact string) bool { return fact == "" })
		described = append(described, iface.Name+" ("+strings.Join(facts, ", ")+")")
	}
	return strings.Join(described, "; ")
}
