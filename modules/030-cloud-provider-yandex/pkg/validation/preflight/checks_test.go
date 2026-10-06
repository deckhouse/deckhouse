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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
)

// testClusterPrefix is a prefix accepted by prefixRegex. dhctl always sends one, so every case
// that is meant to reach the other rules has to carry a valid prefix - otherwise the prefix
// violation alone would make the assertion pass for the wrong reason.
const testClusterPrefix = "test"

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

	result := ValidatePreflight(nil, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidatePreflight(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidatePreflightSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &ycval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}
	if result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix); result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRequiresCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidCredentialSecretType(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].Type = string(corev1.SecretTypeTLS)

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightInvalidCredentialSecretServiceAccount(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].StringData.Secret = "invalid"

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeInvalidServiceAccountSecret) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterNodeGroup(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = nil

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeMasterNodeGroupRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightAllowsNilCloudInstancesOnMaster(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances = nil

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors for master without CloudInstances: %s", result.Error())
	}
}

func TestValidatePreflightRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresExistingInstanceClass(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = nil

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeInstanceClassNotFound) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDiskSizeGB = nil

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsRepeatedProvisionedStorageClassNames(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.Settings.Storage.Parameters.ProvisionedStorageClasses = []ycsettingsv2.ProvisionedStorageClass{
		{Name: "network-ssd-64k", Type: "network-ssd", BlockSize: "64Ki"},
		{Name: "network-ssd-64k", Type: "network-ssd", BlockSize: "128Ki"},
	}

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, ycval.CodeProvisionedStorageClassNamesUnique) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), ycval.CodeProvisionedStorageClassNamesUnique)
	}
}

func TestValidatePreflightAllowsUniqueProvisionedStorageClassNames(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.Settings.Storage.Parameters.ProvisionedStorageClasses = []ycsettingsv2.ProvisionedStorageClass{
		{Name: "network-ssd-64k", Type: "network-ssd", BlockSize: "64Ki"},
		{Name: "network-ssd", Type: "network-ssd", BlockSize: "128Ki"},
	}

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() = %q, want no errors", result.Error())
	}
}

// The check is part of the new-model block, so a cluster still on the legacy PCC with
// migration pending must not be blocked by it.
func TestValidatePreflightSkipsProvisionedStorageClassesDuringMigration(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.MigrationStatus = cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true}
	state.ModuleConfig.Spec.Settings.Storage.Parameters.ProvisionedStorageClasses = []ycsettingsv2.ProvisionedStorageClass{
		{Name: "network-ssd-64k", Type: "network-ssd"},
		{Name: "network-ssd-64k", Type: "network-ssd"},
	}

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightSuccess(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(validState(t), validatev1.OperationBootstrap, testClusterPrefix)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors: %s", result.Error())
	}
}

func TestValidatePreflightInvalidKindStillChecksNameWhenPresent(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = ""

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !strings.Contains(result.Error(), "node_group_class_reference_name_required") &&
		!hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

// The legacy PCC rules run before the migration gate, so a cluster still on the PCC with
// migration pending is checked by them even though the new-model rules are skipped.
func TestValidatePreflightChecksLegacyPCCDuringMigration(t *testing.T) {
	t.Parallel()

	state := &ycval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &ycpccv1.YandexProviderClusterConfiguration{
			Layout: ycval.LayoutWithNATInstance,
			Provider: ycpccv1.YandexProvider{
				ServiceAccountJSON: "invalid",
			},
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
				Replicas: 3,
				InstanceClass: ycpccv1.YandexMasterInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						DiskType:            ptr.To("network-ssd-nonreplicated"),
						DiskSizeGB:          ptr.To(100),
						ExternalIPAddresses: []string{"1.1.1.1"},
					},
				},
			},
		},
	}

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	for _, code := range []string{
		ycval.CodePCCInvalidServiceAccountSecret,
		ycval.CodePCCMasterReplicasGreaterExternalIPAddresses,
		ycval.CodePCCNATInstanceSubnetRequired,
		ycval.CodePCCDiskHasNonMultipleSize,
	} {
		if !hasViolationCode(result, code) {
			t.Errorf("ValidatePreflight() = %q, want %s", result.Error(), code)
		}
	}
}

func TestValidatePreflightAllowsValidLegacyPCC(t *testing.T) {
	t.Parallel()

	state := &ycval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &ycpccv1.YandexProviderClusterConfiguration{
			Layout: "Standard",
			Provider: ycpccv1.YandexProvider{
				ServiceAccountJSON: "{}",
			},
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
				Replicas: 1,
				InstanceClass: ycpccv1.YandexMasterInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						DiskType:   ptr.To("network-ssd-nonreplicated"),
						DiskSizeGB: ptr.To(93),
					},
					EtcdDiskSizeGB: ptr.To(93),
				},
			},
		},
	}

	if result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix); result.HasErrors() {
		t.Fatalf("ValidatePreflight() = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRejectsNonMultipleInstanceClassDiskSize(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.DiskType = "network-ssd-io-m3"
	state.InstanceClasses[0].Spec.DiskSizeGB = 100
	state.InstanceClasses[0].Spec.EtcdDiskSizeGB = ptr.To(93)

	result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix)
	if !hasViolationCode(result, ycval.CodeDiskHasNonMultipleSize) {
		t.Fatalf("ValidatePreflight() = %q, want %s", result.Error(), ycval.CodeDiskHasNonMultipleSize)
	}
}

func TestValidatePreflightAllowsMultipleInstanceClassDiskSize(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.DiskType = "network-ssd-io-m3"
	state.InstanceClasses[0].Spec.DiskSizeGB = 186
	state.InstanceClasses[0].Spec.EtcdDiskSizeGB = ptr.To(93)

	if result := ValidatePreflight(state, validatev1.OperationBootstrap, testClusterPrefix); result.HasErrors() {
		t.Fatalf("ValidatePreflight() = %q, want no errors", result.Error())
	}
}

// The cluster prefix becomes the name prefix of every cloud resource the layouts create, and
// Yandex Cloud rejects names that do not match the pattern. The check has to fail preflight,
// before any infrastructure is touched, rather than mid-apply on the first resource - this is
// the rule the in-tree dhctl validator used to enforce.
func TestValidateClusterPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefix  string
		wantErr bool
	}{
		{name: "lowercase letters", prefix: "cloud"},
		{name: "letters digits and dashes", prefix: "my-cluster-1"},
		{name: "single letter", prefix: "a"},
		// The pattern allows a leading letter, up to 61 middle characters and a final
		// alphanumeric, so 63 is the longest prefix it accepts.
		{name: "63 characters", prefix: "a" + strings.Repeat("b", 62)},
		{name: "64 characters", prefix: "a" + strings.Repeat("b", 63), wantErr: true},
		{name: "empty", prefix: "", wantErr: true},
		{name: "uppercase", prefix: "MyCluster", wantErr: true},
		{name: "underscore", prefix: "k8s_dev", wantErr: true},
		{name: "leading digit", prefix: "1abc", wantErr: true},
		{name: "leading dash", prefix: "-abc", wantErr: true},
		{name: "trailing dash", prefix: "abc-", wantErr: true},
		{name: "dot", prefix: "abc.def", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := validateClusterPrefix(tt.prefix)
			if got := hasViolationCode(result, CodeInvalidClusterPrefix); got != tt.wantErr {
				t.Fatalf("validateClusterPrefix(%q) violation = %v, want %v (%s)", tt.prefix, got, tt.wantErr, result.Error())
			}
		})
	}
}

// The prefix is checked for every cluster, so the violation has to survive both the legacy-PCC
// branch and the migration gate that skips the new-model rules.
func TestValidatePreflightReportsInvalidClusterPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state func(t *testing.T) *ycval.State
	}{
		{
			name:  "new model",
			state: validState,
		},
		{
			name: "migration pending",
			state: func(*testing.T) *ycval.State {
				return &ycval.State{
					MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ValidatePreflight(tt.state(t), validatev1.OperationBootstrap, "Invalid_Prefix")
			if !hasViolationCode(result, CodeInvalidClusterPrefix) {
				t.Fatalf("ValidatePreflight() with an invalid prefix = %q, want %s", result.Error(), CodeInvalidClusterPrefix)
			}
		})
	}
}

// Destroy is exempt: the validator returns before the rules run, so nothing here should depend on
// the prefix. Converge is not - it creates resources just like bootstrap does.
func TestValidatePreflightChecksClusterPrefixOnConverge(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(validState(t), validatev1.OperationConverge, "Invalid_Prefix")
	if !hasViolationCode(result, CodeInvalidClusterPrefix) {
		t.Fatalf("ValidatePreflight() on converge = %q, want %s", result.Error(), CodeInvalidClusterPrefix)
	}
}

// A valid prefix must not add a violation of its own, otherwise every other assertion in this
// file could be passing for the wrong reason.
func TestValidatePreflightAcceptsValidClusterPrefix(t *testing.T) {
	t.Parallel()

	result := ValidatePreflight(validState(t), validatev1.OperationBootstrap, testClusterPrefix)
	if hasViolationCode(result, CodeInvalidClusterPrefix) {
		t.Fatalf("ValidatePreflight() with prefix %q = %q, want no prefix violation", testClusterPrefix, result.Error())
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
					AuthScheme: cpapi.AuthSchemeServiceAccount,
					Secret:     "{}",
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
