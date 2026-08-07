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

package admission

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
)

// yandexInstanceClass builds a YandexInstanceClass with the given name and optional etcd disk.
func yandexInstanceClass(name string, etcdDiskSizeGB *int) *ycicv1.YandexInstanceClass {
	class := &ycicv1.YandexInstanceClass{}
	class.Kind = ycicv1.YandexInstanceClassKind
	class.Name = name
	class.Spec.EtcdDiskSizeGB = etcdDiskSizeGB

	return class
}

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}
	return false
}

func TestValidateInstanceClassAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = append(
		state.InstanceClasses, yandexInstanceClass("orphan-yandex", nil),
	)

	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClass() = %q, want unattached etcdDisk allowed", result.Error())
	}
}

func TestValidateAdmissionSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &ycval.State{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*ycsettingsv2.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: ycmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*ycsettingsv2.ModuleConfigSettings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &ycsettingsv2.ModuleConfigSettings{
					Storage: ycsettingsv2.Storage{Disabled: true},
					Nodes:   ycsettingsv2.Nodes{Disabled: true},
				},
			},
		},
		MigrationStatus: cpapi.MigrationStatus{
			LegacyPCCPresent: true,
			MigrationPending: true,
		},
	}

	for name, validate := range map[string]func(*ycval.State) cpvalapi.Result{
		"ValidateCredentialSecret": func(state *ycval.State) cpvalapi.Result {
			return ValidateCredentialSecret(state, admissionv1.Update)
		},
		"ValidateInstanceClass": func(state *ycval.State) cpvalapi.Result {
			return ValidateInstanceClass(state, admissionv1.Update, nil)
		},
		"ValidateNodeGroup": func(state *ycval.State) cpvalapi.Result {
			return ValidateNodeGroup(state, admissionv1.Update)
		},
	} {
		result := validate(state)
		if result.HasErrors() {
			t.Fatalf("%s() during migration = %q, want no errors", name, result.Error())
		}
	}
}

func TestValidateInstanceClassRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDiskSizeGB = nil

	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidateInstanceClass() = %q, want master etcdDisk requirement", result.Error())
	}
}

func TestValidateNodeGroupAllowsNilCloudInstancesOnCloudPermanentWorker(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = append(
		state.NodeGroups, cpapi.NodeGroup{
			ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
			},
		},
	)

	result := ValidateNodeGroup(state, admissionv1.Update)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroup() unexpected errors for worker without CloudInstances: %s", result.Error())
	}
}

func TestValidateNodeGroupAllowsMissingMasterInstanceClass(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = nil

	result := ValidateNodeGroup(state, admissionv1.Create)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroup(%s) = %q, want allow missing InstanceClass", admissionv1.Create, result.Error())
	}
}

func TestValidateCredentialSecretDoesNotRequirePrimaryCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidateCredentialSecret(state, admissionv1.Update)
	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecret() = %q, want no primary credential requirement", result.Error())
	}
}

func TestValidateNodeGroupRejectsTooFewExternalIPAddresses(t *testing.T) {
	t.Parallel()

	enabled := true
	state := &ycval.State{
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
							ExternalIPAddresses: map[string][]string{"worker": {"1.1.1.1"}},
						},
					},
				},
			},
		},
		NodeGroups: []cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						MaxPerZone: 3,
						ClassReference: &cpapi.ClassReference{
							Kind: ycicv1.YandexInstanceClassKind,
							Name: "worker-yandex",
						},
					},
				},
			},
		},
	}

	result := ValidateNodeGroup(state, admissionv1.Create)
	if !hasViolationCode(result, ycval.CodeNodeGroupNodesGreaterExternalIPAddresses) {
		t.Fatalf("ValidateNodeGroup() = %q, want %s", result.Error(), ycval.CodeNodeGroupNodesGreaterExternalIPAddresses)
	}
}

func validState(t *testing.T) *ycval.State {
	t.Helper()

	etcdDisk := ptr.To(20)
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
						Parameters: ycsettingsv2.ProviderParameters{
							CloudID:  "cloud",
							FolderID: "folder",
						},
					},
					Storage: ycsettingsv2.Storage{Disabled: false},
					Nodes:   ycsettingsv2.Nodes{Disabled: true},
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
							Kind: ycicv1.YandexInstanceClassKind,
							Name: "master-yandex",
						},
					},
				},
			},
		},
		InstanceClasses: []*ycicv1.YandexInstanceClass{
			yandexInstanceClass("master-yandex", etcdDisk),
		},
	}
	return state
}

func validKubeconfigB64ForTest() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu" // gitleaks:allow
}
