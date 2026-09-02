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
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvppccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/pcc/v1"
	dvpsettings "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/settings"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
	"k8s.io/utils/ptr"
)

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, v := range result.Errors() {
		if v.Code == code {
			return true
		}
	}
	return false
}

func dvpInstanceClass(name string, etcdDiskSize string) *dvpicv1alpha1.DVPInstanceClass {
	class := &dvpicv1alpha1.DVPInstanceClass{}
	class.Kind = dvpicv1alpha1.GroupVersionKind.Kind
	class.Name = name
	if etcdDiskSize != "" {
		class.Spec.EtcdDisk.Size = etcdDiskSize
	}
	return class
}

func TestValidatePreflightNilState(t *testing.T) {
	t.Parallel()
	result := ValidatePreflight(nil)
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidatePreflight(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidatePreflightSkipsPendingMigration(t *testing.T) {
	t.Parallel()
	state := &dvpval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}
	if result := ValidatePreflight(state); result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidPCCKubeconfig(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ProviderClusterConfig = &dvppccv1.DVPProviderClusterConfiguration{
		Provider: dvppccv1.DVPProvider{KubeconfigDataBase64: "%%%"},
	}

	result := ValidatePreflight(state)
	if !hasViolationCode(result, CodePCCInvalidKubeconfigSecret) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidPCCKubeconfigDuringMigration(t *testing.T) {
	t.Parallel()

	state := &dvpval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
		ProviderClusterConfig: &dvppccv1.DVPProviderClusterConfiguration{
			Provider: dvppccv1.DVPProvider{KubeconfigDataBase64: "%%%-not-base64"},
		},
	}

	result := ValidatePreflight(state)
	if !hasViolationCode(result, CodePCCInvalidKubeconfigSecret) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresCredentialSecret(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.CredentialSecrets = nil
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

// TestValidatePreflightRequiresManagedCredentialSecret checks that an existing
// Secret with a non-credential type is not treated as the provider credential:
// unmanaged Secrets are filtered out by ListCredentialSecrets, so only the
// "secret is required" violation surfaces.
func TestValidatePreflightRequiresManagedCredentialSecret(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.CredentialSecrets[0].Type = "Opaque"
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeCredentialSecretRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterNodeGroup(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.NodeGroups = nil
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeMasterNodeGroupRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightAllowsNilCloudInstancesOnMaster(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances = nil
	result := ValidatePreflight(state)
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors: %s", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidInstanceClassKind(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeNodeGroupInvalidInstanceClassKind) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightInvalidKindStillChecksNameWhenPresent(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = ""

	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresInstanceClassName(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = "  "
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeNodeGroupClassReferenceNameRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresExistingInstanceClass(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.InstanceClasses = nil
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeInstanceClassNotFound) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDisk.Size = ""
	result := ValidatePreflight(state)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightSuccess(t *testing.T) {
	t.Parallel()
	result := ValidatePreflight(validState(t))
	if result.HasErrors() {
		t.Fatalf("ValidatePreflight() unexpected errors: %s", result.Error())
	}
}

func validState(t *testing.T) *dvpval.State {
	t.Helper()
	enabled := ptr.To(true)
	return &dvpval.State{
		ModuleName:    dvpmeta.ModuleName,
		NamespaceName: dvpmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*dvpsettings.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: dvpmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*dvpsettings.ModuleConfigSettings]{
				Enabled: enabled,
				Version: 2,
				Settings: &dvpsettings.ModuleConfigSettings{
					Provider: dvpsettings.Provider{
						Parameters: dvpsettings.ProviderParameters{Namespace: dvpmeta.Namespace, NetworkPolicy: "Isolated"},
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: dvpmeta.Namespace},
				Type:       cpapi.CredentialsSecretType,
				StringData: cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeKubeconfig,
					Secret:     validKubeconfigB64ForTest(),
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
							Kind: dvpicv1alpha1.GroupVersionKind.Kind,
							Name: "master-dvp",
						},
					},
				},
			},
		},
		InstanceClasses: []*dvpicv1alpha1.DVPInstanceClass{
			dvpInstanceClass("master-dvp", "10Gi"),
		},
	}
}

func validKubeconfigB64ForTest() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu" // gitleaks:allow
}
