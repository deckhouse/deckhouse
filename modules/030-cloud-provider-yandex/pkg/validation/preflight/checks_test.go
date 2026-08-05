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

package preflight

import (
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}
	return false
}

// yandexInstanceClass builds a YandexInstanceClass with the given name and optional etcd disk.
func yandexInstanceClass(name string, etcdDiskSizeGB *int) *ycicv1.YandexInstanceClass {
	class := &ycicv1.YandexInstanceClass{}
	class.Kind = ycicv1.YandexInstanceClassKind
	class.Name = name
	class.Spec.EtcdDiskSizeGB = etcdDiskSizeGB

	return class
}

func TestValidatePreflightNilState(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(nil, proto.OperationBootstrap)
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidatePreflight(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidatePreflightSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &ycval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}
	if result := ValidatePreflight(state, proto.OperationBootstrap); result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRequiresCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidCredentialSecretType(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].Type = string(corev1.SecretTypeTLS)

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterNodeGroup(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = nil

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeMasterNodeGroupRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightAllowsNilCloudInstancesOnMaster(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances = nil

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors for master without CloudInstances: %s", result.Error())
	}
}

func TestValidatePreflightRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresExistingInstanceClass(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = nil

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeInstanceClassNotFound) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDiskSizeGB = nil

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightSuccess(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(validState(t), proto.OperationBootstrap)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors: %s", result.Error())
	}
}

func TestValidatePreflightInvalidKindStillChecksNameWhenPresent(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = ""

	result := ValidatePreflight(state, proto.OperationBootstrap)
	if !strings.Contains(result.Error(), "node_group_class_reference_name_required") &&
		!hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidateMasterNodeGroupReplicasAndIPAddresses(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
			Replicas: 3,
			InstanceClass: ycpccv1.YandexMasterInstanceClass{
				YandexInstanceClass: ycpccv1.YandexInstanceClass{
					ExternalIPAddresses: []string{"1.1.1.1"},
				},
			},
		},
	}

	result := validateMasterNodeGroupReplicasAndIPAddresses(pcc)
	if !hasViolationCode(result, CodePCCMasterReplicasGreaterExternalIPAddresses) {
		t.Fatalf("validateMasterNodeGroupReplicasAndIPAddresses() = %q, want %s", result.Error(), CodePCCMasterReplicasGreaterExternalIPAddresses)
	}
}

func TestValidateNodeGroupsReplicasAndIPAddresses(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		NodeGroups: []ycpccv1.YandexStaticNodeGroup{
			{
				Name:     "worker",
				Replicas: 2,
				InstanceClass: ycpccv1.YandexStaticInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						ExternalIPAddresses: []string{"1.1.1.1"},
					},
				},
			},
		},
	}

	result := validateNodeGroupsReplicasAndIPAddresses(pcc)
	if !hasViolationCode(result, CodePCCNodeGroupReplicasGreaterExternalIPAddresses) {
		t.Fatalf("validateNodeGroupsReplicasAndIPAddresses() = %q, want %s", result.Error(), CodePCCNodeGroupReplicasGreaterExternalIPAddresses)
	}
}

func TestValidateWithNATInstanceLayoutRequiresSubnetOnBootstrap(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		Layout: ycval.LayoutWithNATInstance,
	}

	result := validateWithNATInstanceLayout(pcc, proto.OperationBootstrap)
	if !hasViolationCode(result, CodePCCNATInstanceSubnetRequired) {
		t.Fatalf("validateWithNATInstanceLayout() = %q, want %s", result.Error(), CodePCCNATInstanceSubnetRequired)
	}
}

func TestValidateWithNATInstanceLayoutSkipsCheckOnConverge(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{
		Layout: ycval.LayoutWithNATInstance,
	}

	if result := validateWithNATInstanceLayout(pcc, proto.OperationConverge); result.HasErrors() {
		t.Fatalf("validateWithNATInstanceLayout() on converge = %q, want no errors", result.Error())
	}
}

func TestPCCChecksEmptyPCC(t *testing.T) {
	t.Parallel()

	pcc := &ycpccv1.YandexProviderClusterConfiguration{}

	result := cpvalapi.Result{}
	result.Merge(
		validateMasterNodeGroupReplicasAndIPAddresses(pcc),
		validateNodeGroupsReplicasAndIPAddresses(pcc),
		validateWithNATInstanceLayout(pcc, proto.OperationBootstrap),
	)
	if result.HasErrors() {
		t.Fatalf("PCC checks on empty PCC = %q, want no errors", result.Error())
	}
}

func validState(t *testing.T) *ycval.State {
	t.Helper()

	state := &ycval.State{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*ycsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: ycmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*ycsettingsv2.ModuleConfigSettings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &ycsettingsv2.ModuleConfigSettings{
					Provider: ycsettingsv2.Provider{
						Parameters: ycsettingsv2.ProviderParameters{CloudID: "cloud", FolderID: "folder"},
					},
					Nodes: ycsettingsv2.Nodes{
						Parameters: ycsettingsv2.NodesParameters{SSHPublicKey: "ssh-rsa AAA", Layout: "Standard"},
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{
					Name:      cpapi.CredentialSecretName,
					Namespace: ycmeta.Namespace,
				},
				Type: cpapi.CredentialsSecretType,
				StringData: cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeAPIToken,
					Secret:     "test-token",
				},
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: ycicv1.YandexInstanceClassKind,
							Name: "master-yandex",
						},
					},
				},
			},
		},
		InstanceClasses: []*ycicv1.YandexInstanceClass{
			yandexInstanceClass("master-yandex", ptr.To(20)),
		},
	}

	return state
}
