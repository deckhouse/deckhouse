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
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
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
