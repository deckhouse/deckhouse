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

package validation

import (
	"strconv"
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}

	return false
}

// externalIPState builds a validation state with one CloudPermanent NodeGroup and
// the given externalIPAddresses map in ModuleConfig settings.
func externalIPState(nodeGroupName string, maxPerZone int, externalIPAddresses map[string][]string) *State {
	enabled := true

	return &State{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*ycsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: ycmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*ycsettingsv2.ModuleConfigSettings]{
				Enabled: &enabled,
				Version: 2,
				Settings: &ycsettingsv2.ModuleConfigSettings{
					Nodes: ycsettingsv2.Nodes{
						Parameters: ycsettingsv2.NodesParameters{
							ExternalIPAddresses: externalIPAddresses,
						},
					},
				},
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: nodeGroupName},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						MaxPerZone: maxPerZone,
						MinPerZone: maxPerZone,
						ClassReference: &cpapi.ClassReference{
							Kind: ycicv1.YandexInstanceClassKind,
							Name: cpapi.BuildInstanceClassName(nodeGroupName),
						},
					},
				},
			},
		},
	}
}

// provisionedStorageClassesState builds a validation state whose ModuleConfig carries the given
// storage.parameters.provisionedStorageClasses list and nothing else.
func provisionedStorageClassesState(storageClasses ...ycsettingsv2.ProvisionedStorageClass) *State {
	enabled := true

	return &State{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*ycsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: ycmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*ycsettingsv2.ModuleConfigSettings]{
				Enabled: &enabled,
				Version: 2,
				Settings: &ycsettingsv2.ModuleConfigSettings{
					Storage: ycsettingsv2.Storage{
						Parameters: ycsettingsv2.StorageParameters{
							ProvisionedStorageClasses: storageClasses,
						},
					},
				},
			},
		},
	}
}

func TestValidateProvisionedStorageClassesRejectsRepeatedNames(t *testing.T) {
	t.Parallel()

	state := provisionedStorageClassesState(
		ycsettingsv2.ProvisionedStorageClass{Name: "network-ssd-64k", Type: "network-ssd", BlockSize: "64Ki"},
		ycsettingsv2.ProvisionedStorageClass{Name: "network-ssd-64k", Type: "network-ssd-nonreplicated"},
	)

	result := ValidateProvisionedStorageClasses(state)
	if !hasViolationCode(result, CodeProvisionedStorageClassNamesUnique) {
		t.Fatalf("ValidateProvisionedStorageClasses() = %q, want %s", result.Error(), CodeProvisionedStorageClassNamesUnique)
	}
}

// A repeated name is reported once, carrying the first colliding name as the rejected value:
// Result keys violations by code and path, so a later duplicate would overwrite the message.
func TestValidateProvisionedStorageClassesReportsFirstRepeatedNameOnce(t *testing.T) {
	t.Parallel()

	state := provisionedStorageClassesState(
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-a", Type: "network-ssd"},
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-b", Type: "network-ssd"},
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-a", Type: "network-hdd"},
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-b", Type: "network-hdd"},
	)

	violations := ValidateProvisionedStorageClasses(state).Errors()
	if len(violations) != 1 {
		t.Fatalf("ValidateProvisionedStorageClasses() returned %d violations, want 1", len(violations))
	}
	if violations[0].Value != "sc-a" {
		t.Fatalf("violation value = %v, want sc-a", violations[0].Value)
	}
	if violations[0].Path != "ModuleConfig.spec.settings.storage.parameters.provisionedStorageClasses" {
		t.Fatalf("violation path = %q, want the provisionedStorageClasses path", violations[0].Path)
	}
}

func TestValidateProvisionedStorageClassesAllowsUniqueNames(t *testing.T) {
	t.Parallel()

	state := provisionedStorageClassesState(
		ycsettingsv2.ProvisionedStorageClass{Name: "network-ssd-64k", Type: "network-ssd", BlockSize: "64Ki"},
		ycsettingsv2.ProvisionedStorageClass{Name: "network-ssd-128k", Type: "network-ssd", BlockSize: "128Ki"},
	)

	if result := ValidateProvisionedStorageClasses(state); result.HasErrors() {
		t.Fatalf("ValidateProvisionedStorageClasses() = %q, want no errors", result.Error())
	}
}

// Overriding a StorageClass created by default is the documented purpose of the parameter,
// not a duplicate: hooks/storage_classes.go drops the default in favour of the provisioned entry.
func TestValidateProvisionedStorageClassesAllowsOverridingDefaultNames(t *testing.T) {
	t.Parallel()

	state := provisionedStorageClassesState(
		ycsettingsv2.ProvisionedStorageClass{Name: "network-ssd", Type: "network-ssd", BlockSize: "64Ki"},
		ycsettingsv2.ProvisionedStorageClass{Name: "network-hdd", Type: "network-hdd", BlockSize: "32Ki"},
	)

	if result := ValidateProvisionedStorageClasses(state); result.HasErrors() {
		t.Fatalf("ValidateProvisionedStorageClasses() = %q, want no errors", result.Error())
	}
}

func TestValidateProvisionedStorageClassesAllowsEmptyList(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]*State{
		"nil list":   provisionedStorageClassesState(),
		"empty list": provisionedStorageClassesState([]ycsettingsv2.ProvisionedStorageClass{}...),
	} {
		if result := ValidateProvisionedStorageClasses(state); result.HasErrors() {
			t.Fatalf("ValidateProvisionedStorageClasses() with %s = %q, want no errors", name, result.Error())
		}
	}
}

func TestValidateProvisionedStorageClassesWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := provisionedStorageClassesState(
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-a", Type: "network-ssd"},
		ycsettingsv2.ProvisionedStorageClass{Name: "sc-a", Type: "network-ssd"},
	)
	state.ModuleConfig = nil

	if result := ValidateProvisionedStorageClasses(state); result.HasErrors() {
		t.Fatalf("ValidateProvisionedStorageClasses() = %q, want no errors", result.Error())
	}
}

func TestValidateProvisionedStorageClassesNilState(t *testing.T) {
	t.Parallel()

	if result := ValidateProvisionedStorageClasses(nil); !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateProvisionedStorageClasses(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidateNodeGroupExternalIPAddressesRejectsTooFewAddresses(t *testing.T) {
	t.Parallel()

	state := externalIPState("master", 3, map[string][]string{"master": {"1.1.1.1", "2.2.2.2"}})

	result := ValidateNodeGroupExternalIPAddresses(state)
	if !hasViolationCode(result, CodeNodeGroupNodesGreaterExternalIPAddresses) {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses() = %q, want %s", result.Error(), CodeNodeGroupNodesGreaterExternalIPAddresses)
	}
}

func TestValidateNodeGroupExternalIPAddressesAllowsEnoughAddresses(t *testing.T) {
	t.Parallel()

	state := externalIPState("master", 3, map[string][]string{"master": {"1.1.1.1", "2.2.2.2", "Auto"}})

	if result := ValidateNodeGroupExternalIPAddresses(state); result.HasErrors() {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses() = %q, want no errors", result.Error())
	}
}

func TestValidateNodeGroupExternalIPAddressesSkipsGroupsWithoutAddresses(t *testing.T) {
	t.Parallel()

	state := externalIPState("worker", 5, map[string][]string{"master": {"1.1.1.1"}})

	if result := ValidateNodeGroupExternalIPAddresses(state); result.HasErrors() {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses() = %q, want no errors", result.Error())
	}
}

func TestValidateNodeGroupExternalIPAddressesSkipsNonCloudPermanent(t *testing.T) {
	t.Parallel()

	state := externalIPState("worker", 5, map[string][]string{"worker": {"1.1.1.1"}})
	state.NodeGroups[0].Spec.NodeType = "Static"

	if result := ValidateNodeGroupExternalIPAddresses(state); result.HasErrors() {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses() = %q, want no errors", result.Error())
	}
}

func TestValidateNodeGroupExternalIPAddressesWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := externalIPState("master", 3, map[string][]string{"master": {"1.1.1.1"}})
	state.ModuleConfig = nil

	if result := ValidateNodeGroupExternalIPAddresses(state); result.HasErrors() {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses() = %q, want no errors", result.Error())
	}
}

func TestValidateNodeGroupExternalIPAddressesNilState(t *testing.T) {
	t.Parallel()

	if result := ValidateNodeGroupExternalIPAddresses(nil); !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateNodeGroupExternalIPAddresses(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

// natInstanceLayoutState builds a validation state whose ModuleConfig carries the given layout and
// withNATInstance parameters and nothing else.
func natInstanceLayoutState(layout string, natInstance ycsettingsv2.NATInstanceParameters) *State {
	enabled := true

	return &State{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*ycsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: ycmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*ycsettingsv2.ModuleConfigSettings]{
				Enabled: &enabled,
				Version: 2,
				Settings: &ycsettingsv2.ModuleConfigSettings{
					Nodes: ycsettingsv2.Nodes{
						Parameters: ycsettingsv2.NodesParameters{
							Layout:          layout,
							WithNATInstance: natInstance,
						},
					},
				},
			},
		},
	}
}

func TestValidateWithNATInstanceLayoutRequiresSubnet(t *testing.T) {
	t.Parallel()

	state := natInstanceLayoutState(LayoutWithNATInstance, ycsettingsv2.NATInstanceParameters{
		ExternalSubnetID: "external",
	})

	result := ValidateWithNATInstanceLayout(state)
	if !hasViolationCode(result, CodeNATInstanceSubnetRequired) {
		t.Fatalf("ValidateWithNATInstanceLayout() = %q, want %s", result.Error(), CodeNATInstanceSubnetRequired)
	}
}

func TestValidateWithNATInstanceLayoutAcceptsInternalSubnetID(t *testing.T) {
	t.Parallel()

	state := natInstanceLayoutState(LayoutWithNATInstance, ycsettingsv2.NATInstanceParameters{
		InternalSubnetID: "subnet-id",
	})

	if result := ValidateWithNATInstanceLayout(state); result.HasErrors() {
		t.Fatalf("ValidateWithNATInstanceLayout() = %q, want no errors", result.Error())
	}
}

func TestValidateWithNATInstanceLayoutAcceptsInternalSubnetCIDR(t *testing.T) {
	t.Parallel()

	state := natInstanceLayoutState(LayoutWithNATInstance, ycsettingsv2.NATInstanceParameters{
		InternalSubnetCIDR: "10.0.0.0/24",
	})

	if result := ValidateWithNATInstanceLayout(state); result.HasErrors() {
		t.Fatalf("ValidateWithNATInstanceLayout() = %q, want no errors", result.Error())
	}
}

// Only the WithNATInstance layout carries the requirement: an empty withNATInstance section is
// the norm for every other layout.
func TestValidateWithNATInstanceLayoutSkipsOtherLayouts(t *testing.T) {
	t.Parallel()

	for _, layout := range []string{"Standard", "WithoutNAT", ""} {
		state := natInstanceLayoutState(layout, ycsettingsv2.NATInstanceParameters{})

		if result := ValidateWithNATInstanceLayout(state); result.HasErrors() {
			t.Fatalf("ValidateWithNATInstanceLayout(layout=%q) = %q, want no errors", layout, result.Error())
		}
	}
}

// The reported value must not carry anything sensitive: NATInstanceParameters holds subnet and
// address fields only, while the exporter API key lives in a credential Secret.
func TestValidateWithNATInstanceLayoutReportsSubnetFieldsOnly(t *testing.T) {
	t.Parallel()

	natInstance := ycsettingsv2.NATInstanceParameters{ExternalSubnetID: "external"}
	violations := ValidateWithNATInstanceLayout(
		natInstanceLayoutState(LayoutWithNATInstance, natInstance),
	).Errors()

	if len(violations) != 1 {
		t.Fatalf("ValidateWithNATInstanceLayout() returned %d violations, want 1", len(violations))
	}
	if violations[0].Value != natInstance {
		t.Fatalf("violation value = %v, want %v", violations[0].Value, natInstance)
	}
}

func TestValidateWithNATInstanceLayoutWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := natInstanceLayoutState(LayoutWithNATInstance, ycsettingsv2.NATInstanceParameters{})
	state.ModuleConfig = nil

	if result := ValidateWithNATInstanceLayout(state); result.HasErrors() {
		t.Fatalf("ValidateWithNATInstanceLayout() = %q, want no errors", result.Error())
	}
}

func TestValidateWithNATInstanceLayoutNilState(t *testing.T) {
	t.Parallel()

	if result := ValidateWithNATInstanceLayout(nil); !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateWithNATInstanceLayout(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

// diskInstanceClass builds a YandexInstanceClass with the given disk type, disk size and optional etcd disk.
func diskInstanceClass(name, diskType string, diskSizeGB int, etcdDiskSizeGB *int) *ycicv1.YandexInstanceClass {
	class := &ycicv1.YandexInstanceClass{}
	class.Kind = ycicv1.YandexInstanceClassKind
	class.Name = name
	class.Spec.DiskType = diskType
	class.Spec.DiskSizeGB = diskSizeGB
	class.Spec.EtcdDiskSizeGB = etcdDiskSizeGB

	return class
}

func violationPaths(result cpvalapi.Result, code string) []string {
	var paths []string
	for _, violation := range result.Errors() {
		if violation.Code == code {
			paths = append(paths, violation.Path)
		}
	}

	return paths
}

func assertViolationPaths(t *testing.T, result cpvalapi.Result, code string, want []string) {
	t.Helper()

	got := violationPaths(result, code)
	if len(got) != len(want) {
		t.Fatalf("%s violations at %v, want %v (%s)", code, got, want, result.Error())
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, path := range got {
		gotSet[path] = struct{}{}
	}
	for _, path := range want {
		if _, ok := gotSet[path]; !ok {
			t.Fatalf("%s violations at %v, want %v (%s)", code, got, want, result.Error())
		}
	}
}

// Yandex Cloud only creates network-ssd-nonreplicated and network-ssd-io-m3 disks whose size is
// a multiple of 93 GiB; diskSizeGB and etcdDiskSizeGB are in GiB.
func TestValidateDiskSizeByType(t *testing.T) {
	t.Parallel()

	const (
		diskPath = "YandexInstanceClass/test.spec.diskSizeGB"
		etcdPath = "YandexInstanceClass/test.spec.etcdDiskSizeGB"
	)

	tests := []struct {
		name           string
		diskType       string
		diskSizeGB     int
		etcdDiskSizeGB *int
		wantPaths      []string
	}{
		{name: "nonreplicated multiple of 93", diskType: "network-ssd-nonreplicated", diskSizeGB: 93},
		{name: "io-m3 multiple of 93", diskType: "network-ssd-io-m3", diskSizeGB: 186, etcdDiskSizeGB: ptr.To(93)},
		{name: "nonreplicated not multiple", diskType: "network-ssd-nonreplicated", diskSizeGB: 100, wantPaths: []string{diskPath}},
		{name: "io-m3 not multiple", diskType: "network-ssd-io-m3", diskSizeGB: 50, wantPaths: []string{diskPath}},
		{name: "etcd disk not multiple", diskType: "network-ssd-nonreplicated", diskSizeGB: 93, etcdDiskSizeGB: ptr.To(20), wantPaths: []string{etcdPath}},
		{name: "both disks not multiple", diskType: "network-ssd-io-m3", diskSizeGB: 94, etcdDiskSizeGB: ptr.To(92), wantPaths: []string{diskPath, etcdPath}},
		{name: "network-ssd any size", diskType: "network-ssd", diskSizeGB: 50, etcdDiskSizeGB: ptr.To(20)},
		{name: "network-hdd any size", diskType: "network-hdd", diskSizeGB: 51},
		{name: "unset disk type", diskSizeGB: 51, etcdDiskSizeGB: ptr.To(10)},
		// An unset diskSizeGB becomes 50 GiB in terraform, MCM and CAPI alike.
		{name: "nonreplicated with unset size", diskType: "network-ssd-nonreplicated", wantPaths: []string{diskPath}},
		{name: "network-ssd with unset size", diskType: "network-ssd"},
		// Only a master class gets an etcd disk, and ValidateInstanceClassesEtcdDisk requires it there.
		{name: "unset etcd disk", diskType: "network-ssd-io-m3", diskSizeGB: 93},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := &State{
				InstanceClasses: []*ycicv1.YandexInstanceClass{
					diskInstanceClass("test", tt.diskType, tt.diskSizeGB, tt.etcdDiskSizeGB),
				},
			}

			assertViolationPaths(t, ValidateDiskSizeByType(state), CodeDiskHasNonMultipleSize, tt.wantPaths)
		})
	}
}

// Every InstanceClass in the state is checked, and each violation names the class it belongs to.
func TestValidateDiskSizeByTypeReportsEveryInstanceClass(t *testing.T) {
	t.Parallel()

	state := &State{
		InstanceClasses: []*ycicv1.YandexInstanceClass{
			diskInstanceClass("valid", "network-ssd-nonreplicated", 93, nil),
			diskInstanceClass("first", "network-ssd-nonreplicated", 100, nil),
			diskInstanceClass("second", "network-ssd-io-m3", 200, nil),
		},
	}

	assertViolationPaths(t, ValidateDiskSizeByType(state), CodeDiskHasNonMultipleSize, []string{
		"YandexInstanceClass/first.spec.diskSizeGB",
		"YandexInstanceClass/second.spec.diskSizeGB",
	})
}

func TestValidateDiskSizeByTypeReportsSizeAndRequiredMultiple(t *testing.T) {
	t.Parallel()

	state := &State{
		InstanceClasses: []*ycicv1.YandexInstanceClass{
			diskInstanceClass("test", "network-ssd-nonreplicated", 100, nil),
		},
	}

	violations := ValidateDiskSizeByType(state).Errors()
	if len(violations) != 1 {
		t.Fatalf("ValidateDiskSizeByType() returned %d violations, want 1", len(violations))
	}
	if violations[0].Value != 100 {
		t.Fatalf("violation value = %v, want 100", violations[0].Value)
	}
	if !strings.Contains(violations[0].Message, "network-ssd-nonreplicated") || !strings.Contains(violations[0].Message, "93 GiB") {
		t.Fatalf("violation message = %q, want the disk type and the 93 GiB multiple", violations[0].Message)
	}
	if strings.Contains(violations[0].Message, "default") {
		t.Fatalf("violation message = %q, want no mention of the default for an explicit size", violations[0].Message)
	}
}

// A violation on a size the user never set has to say where the size came from.
func TestValidateDiskSizeByTypeReportsDefaultSize(t *testing.T) {
	t.Parallel()

	state := &State{
		InstanceClasses: []*ycicv1.YandexInstanceClass{
			diskInstanceClass("test", "network-ssd-io-m3", 0, nil),
		},
	}

	violations := ValidateDiskSizeByType(state).Errors()
	if len(violations) != 1 {
		t.Fatalf("ValidateDiskSizeByType() returned %d violations, want 1", len(violations))
	}
	if violations[0].Value != 50 {
		t.Fatalf("violation value = %v, want the 50 GiB default", violations[0].Value)
	}
	if !strings.Contains(violations[0].Message, "default") {
		t.Fatalf("violation message = %q, want a mention of the default", violations[0].Message)
	}
}

func TestValidateDiskSizeByTypeWithoutInstanceClasses(t *testing.T) {
	t.Parallel()

	if result := ValidateDiskSizeByType(&State{}); result.HasErrors() {
		t.Fatalf("ValidateDiskSizeByType() = %q, want no errors", result.Error())
	}
}

func TestValidateDiskSizeByTypeNilState(t *testing.T) {
	t.Parallel()

	if result := ValidateDiskSizeByType(nil); !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateDiskSizeByType(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidatePCCServiceAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		serviceAccountJSON string
		wantErr            bool
	}{
		{name: "valid json", serviceAccountJSON: "{}"},
		{name: "invalid json", serviceAccountJSON: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pcc := &ycpccv1.YandexProviderClusterConfiguration{
				Provider: ycpccv1.YandexProvider{ServiceAccountJSON: tt.serviceAccountJSON},
			}

			result := ValidatePCCServiceAccount(pcc)
			if got := hasViolationCode(result, CodePCCInvalidServiceAccountSecret); got != tt.wantErr {
				t.Fatalf("ValidatePCCServiceAccount() violation = %v, want %v (%s)", got, tt.wantErr, result.Error())
			}
		})
	}
}

// The service account JSON carries a private key, so the reported value must never be the input.
func TestValidatePCCServiceAccountMasksValue(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		Provider: ycpccv1.YandexProvider{ServiceAccountJSON: "secret-material"},
	}

	violations := ValidatePCCServiceAccount(pcc).Errors()
	if len(violations) != 1 {
		t.Fatalf("ValidatePCCServiceAccount() returned %d violations, want 1", len(violations))
	}
	if violations[0].Value != "masked" {
		t.Fatalf("violation value = %v, want masked", violations[0].Value)
	}
}

func TestValidatePCCNodeGroupsReplicasAndIPAddresses(t *testing.T) {
	t.Parallel()

	const (
		masterPath = "ProviderClusterConfiguration.masterNodeGroup.instanceClass.externalIPAddresses"
		workerPath = "ProviderClusterConfiguration.nodeGroups[1].instanceClass.externalIPAddresses"
	)

	nodeGroup := func(name string, replicas int, addresses ...string) ycpccv1.YandexStaticNodeGroup {
		return ycpccv1.YandexStaticNodeGroup{
			Name:     name,
			Replicas: replicas,
			InstanceClass: ycpccv1.YandexStaticInstanceClass{
				YandexInstanceClass: ycpccv1.YandexInstanceClass{ExternalIPAddresses: addresses},
			},
		}
	}

	tests := []struct {
		name            string
		masterReplicas  int
		masterAddresses []string
		nodeGroups      []ycpccv1.YandexStaticNodeGroup
		wantMasterPaths []string
		wantPaths       []string
	}{
		{
			name:            "master replicas greater than addresses",
			masterReplicas:  3,
			masterAddresses: []string{"1.1.1.1"},
			wantMasterPaths: []string{masterPath},
		},
		{
			name:            "master replicas equal to addresses",
			masterReplicas:  2,
			masterAddresses: []string{"1.1.1.1", "2.2.2.2"},
		},
		{
			name:           "master without addresses",
			masterReplicas: 3,
		},
		{
			name: "node group replicas greater than addresses",
			nodeGroups: []ycpccv1.YandexStaticNodeGroup{
				nodeGroup("frontend", 1, "1.1.1.1"),
				nodeGroup("worker", 2, "2.2.2.2"),
			},
			wantPaths: []string{workerPath},
		},
		{
			name: "node group without addresses",
			nodeGroups: []ycpccv1.YandexStaticNodeGroup{
				nodeGroup("frontend", 1, "1.1.1.1"),
				nodeGroup("worker", 5),
			},
		},
		{
			name:            "master and node group both short of addresses",
			masterReplicas:  3,
			masterAddresses: []string{"1.1.1.1"},
			nodeGroups: []ycpccv1.YandexStaticNodeGroup{
				nodeGroup("frontend", 0, "1.1.1.1"),
				nodeGroup("worker", 2, "2.2.2.2"),
			},
			wantMasterPaths: []string{masterPath},
			wantPaths:       []string{workerPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pcc := &ycpccv1.YandexProviderClusterConfiguration{
				MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
					Replicas: tt.masterReplicas,
					InstanceClass: ycpccv1.YandexMasterInstanceClass{
						YandexInstanceClass: ycpccv1.YandexInstanceClass{ExternalIPAddresses: tt.masterAddresses},
					},
				},
				NodeGroups: tt.nodeGroups,
			}

			result := ValidatePCCNodeGroupsReplicasAndIPAddresses(pcc)
			assertViolationPaths(t, result, CodePCCMasterReplicasGreaterExternalIPAddresses, tt.wantMasterPaths)
			assertViolationPaths(t, result, CodePCCNodeGroupReplicasGreaterExternalIPAddresses, tt.wantPaths)
		})
	}
}

func TestValidatePCCWithNATInstanceLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		layout      string
		operation   validatev1.Operation
		natInstance *ycpccv1.YandexWithNATInstance
		wantErr     bool
	}{
		{
			name:      "no withNATInstance section on bootstrap",
			layout:    LayoutWithNATInstance,
			operation: validatev1.OperationBootstrap,
			wantErr:   true,
		},
		{
			name:        "empty subnet fields on bootstrap",
			layout:      LayoutWithNATInstance,
			operation:   validatev1.OperationBootstrap,
			natInstance: &ycpccv1.YandexWithNATInstance{InternalSubnetID: ptr.To(""), InternalSubnetCIDR: ptr.To("")},
			wantErr:     true,
		},
		{
			name:        "internal subnet ID on bootstrap",
			layout:      LayoutWithNATInstance,
			operation:   validatev1.OperationBootstrap,
			natInstance: &ycpccv1.YandexWithNATInstance{InternalSubnetID: ptr.To("subnet-id")},
		},
		{
			name:        "internal subnet CIDR on bootstrap",
			layout:      LayoutWithNATInstance,
			operation:   validatev1.OperationBootstrap,
			natInstance: &ycpccv1.YandexWithNATInstance{InternalSubnetCIDR: ptr.To("10.0.0.0/24")},
		},
		{
			name:      "converge is not checked",
			layout:    LayoutWithNATInstance,
			operation: validatev1.OperationConverge,
		},
		{
			name:      "other layout",
			layout:    "Standard",
			operation: validatev1.OperationBootstrap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pcc := &ycpccv1.YandexProviderClusterConfiguration{
				Layout:          tt.layout,
				WithNATInstance: tt.natInstance,
			}

			result := ValidatePCCWithNATInstanceLayout(pcc, tt.operation)
			if got := hasViolationCode(result, CodePCCNATInstanceSubnetRequired); got != tt.wantErr {
				t.Fatalf("ValidatePCCWithNATInstanceLayout() violation = %v, want %v (%s)", got, tt.wantErr, result.Error())
			}
		})
	}
}

// The violation goes back to dhctl, so exporterAPIKey must be dropped from the reported section
// while the rest of it is kept for context.
func TestValidatePCCWithNATInstanceLayoutMasksExporterAPIKey(t *testing.T) {
	t.Parallel()

	natInstance := &ycpccv1.YandexWithNATInstance{
		ExporterAPIKey:   ptr.To("secret-key"),
		ExternalSubnetID: ptr.To("external"),
	}
	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		Layout:          LayoutWithNATInstance,
		WithNATInstance: natInstance,
	}

	violations := ValidatePCCWithNATInstanceLayout(pcc, validatev1.OperationBootstrap).Errors()
	if len(violations) != 1 {
		t.Fatalf("ValidatePCCWithNATInstanceLayout() returned %d violations, want 1", len(violations))
	}

	value, ok := violations[0].Value.(*ycpccv1.YandexWithNATInstance)
	if !ok {
		t.Fatalf("violation value = %T, want *YandexWithNATInstance", violations[0].Value)
	}
	if value.ExporterAPIKey != nil {
		t.Fatalf("violation value carries exporterAPIKey %q", *value.ExporterAPIKey)
	}
	if value.ExternalSubnetID == nil || *value.ExternalSubnetID != "external" {
		t.Fatalf("violation value lost externalSubnetID: %+v", value)
	}
	if natInstance.ExporterAPIKey == nil {
		t.Fatalf("ValidatePCCWithNATInstanceLayout() cleared exporterAPIKey in the PCC itself")
	}
}

func TestValidatePCCDiskSizeByType(t *testing.T) {
	t.Parallel()

	const (
		masterDiskPath = "ProviderClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGB"
		masterEtcdPath = "ProviderClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb"
		workerDiskPath = "ProviderClusterConfiguration.nodeGroups[1].instanceClass.diskSizeGB"
	)

	instanceClass := func(diskType *string, diskSizeGB *int) ycpccv1.YandexInstanceClass {
		return ycpccv1.YandexInstanceClass{DiskType: diskType, DiskSizeGB: diskSizeGB}
	}

	tests := []struct {
		name       string
		master     ycpccv1.YandexMasterInstanceClass
		nodeGroups []ycpccv1.YandexInstanceClass
		wantPaths  []string
	}{
		{
			name: "master disks multiple of 93",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd-nonreplicated"), ptr.To(186)),
				EtcdDiskSizeGB:      ptr.To(93),
			},
		},
		{
			name: "master disk not multiple",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd-io-m3"), ptr.To(100)),
				EtcdDiskSizeGB:      ptr.To(93),
			},
			wantPaths: []string{masterDiskPath},
		},
		{
			name: "master etcd disk not multiple",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd-nonreplicated"), ptr.To(93)),
				EtcdDiskSizeGB:      ptr.To(20),
			},
			wantPaths: []string{masterEtcdPath},
		},
		{
			name: "master disk type without size restriction",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd"), ptr.To(100)),
				EtcdDiskSizeGB:      ptr.To(20),
			},
		},
		{
			name: "master without disk type",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(nil, ptr.To(100)),
				EtcdDiskSizeGB:      ptr.To(20),
			},
		},
		{
			// The PCC is projected with 50 GiB and 10 GiB when the sizes are unset.
			name: "master sizes unset",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd-nonreplicated"), nil),
			},
			wantPaths: []string{masterDiskPath, masterEtcdPath},
		},
		{
			name: "master sizes unset for type without size restriction",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd"), nil),
			},
		},
		{
			name: "node group size unset",
			nodeGroups: []ycpccv1.YandexInstanceClass{
				instanceClass(ptr.To("network-hdd"), nil),
				instanceClass(ptr.To("network-ssd-io-m3"), nil),
			},
			wantPaths: []string{workerDiskPath},
		},
		{
			name: "node group disk not multiple",
			nodeGroups: []ycpccv1.YandexInstanceClass{
				instanceClass(ptr.To("network-ssd-nonreplicated"), ptr.To(93)),
				instanceClass(ptr.To("network-ssd-io-m3"), ptr.To(100)),
			},
			wantPaths: []string{workerDiskPath},
		},
		{
			name: "node group disk type without size restriction",
			nodeGroups: []ycpccv1.YandexInstanceClass{
				instanceClass(ptr.To("network-hdd"), ptr.To(100)),
			},
		},
		{
			name: "master and node group disks not multiple",
			master: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: instanceClass(ptr.To("network-ssd-nonreplicated"), ptr.To(50)),
				EtcdDiskSizeGB:      ptr.To(10),
			},
			nodeGroups: []ycpccv1.YandexInstanceClass{
				instanceClass(nil, ptr.To(100)),
				instanceClass(ptr.To("network-ssd-nonreplicated"), ptr.To(100)),
			},
			wantPaths: []string{masterDiskPath, masterEtcdPath, workerDiskPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pcc := &ycpccv1.YandexProviderClusterConfiguration{
				MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{InstanceClass: tt.master},
			}
			for i, instanceClass := range tt.nodeGroups {
				pcc.NodeGroups = append(pcc.NodeGroups, ycpccv1.YandexStaticNodeGroup{
					Name:          "ng-" + strconv.Itoa(i),
					InstanceClass: ycpccv1.YandexStaticInstanceClass{YandexInstanceClass: instanceClass},
				})
			}

			assertViolationPaths(t, ValidatePCCDiskSizeByType(pcc), CodePCCDiskHasNonMultipleSize, tt.wantPaths)
		})
	}
}

// An empty PCC is what a cluster without nodes or optional sections parses to: none of the
// legacy rules may fire on absent fields.
func TestPCCChecksEmptyPCC(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{}

	result := cpvalapi.Result{}
	result.Merge(
		ValidatePCCNodeGroupsReplicasAndIPAddresses(pcc),
		ValidatePCCWithNATInstanceLayout(pcc, validatev1.OperationBootstrap),
		ValidatePCCDiskSizeByType(pcc),
	)
	if result.HasErrors() {
		t.Fatalf("PCC checks on empty PCC = %q, want no errors", result.Error())
	}
}
