// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvpsettings "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/settings"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
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

func TestValidateInstanceClassAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.InstanceClasses = append(state.InstanceClasses, dvpInstanceClass("orphan-dvp", ""))
	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClass() = %q, want unattached etcdDisk allowed", result.Error())
	}
}

func TestValidateAdmissionSkipsPendingMigration(t *testing.T) {
	t.Parallel()
	state := &dvpval.State{
		ModuleName:    dvpmeta.ModuleName,
		NamespaceName: dvpmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*dvpsettings.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: dvpmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*dvpsettings.ModuleConfigSettings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &dvpsettings.ModuleConfigSettings{
					Storage: dvpsettings.Storage{Disabled: true},
					Nodes:   dvpsettings.Nodes{Disabled: true},
				},
			},
		},
		MigrationStatus: cpapi.MigrationStatus{LegacyPCCPresent: true, MigrationPending: true},
	}
	for name, fn := range map[string]func(*dvpval.State) cpvalapi.Result{
		"ValidateCredentialSecret": func(s *dvpval.State) cpvalapi.Result {
			return ValidateCredentialSecret(s, admissionv1.Update)
		},
		"ValidateInstanceClass": func(s *dvpval.State) cpvalapi.Result {
			return ValidateInstanceClass(s, admissionv1.Update, nil)
		},
		"ValidateNodeGroup": func(s *dvpval.State) cpvalapi.Result {
			return ValidateNodeGroup(s, admissionv1.Update)
		},
	} {
		if r := fn(state); r.HasErrors() {
			t.Fatalf("%s() during migration = %q, want no errors", name, r.Error())
		}
	}
}

func TestValidateInstanceClassRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDisk.Size = ""
	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidateInstanceClass() = %q, want master etcdDisk requirement", result.Error())
	}
}

func TestValidateNodeGroupAllowsNilCloudInstancesOnCloudPermanentWorker(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.NodeGroups = append(state.NodeGroups, cpapi.NodeGroup{
		ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
		Spec:       cpapi.NodeGroupSpec{NodeType: cpapi.NodeTypeCloudPermanent},
	})
	if r := ValidateNodeGroup(state, admissionv1.Update); r.HasErrors() {
		t.Fatalf("ValidateNodeGroup() unexpected errors: %s", r.Error())
	}
}

func TestValidateNodeGroupAllowsMissingMasterInstanceClass(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.InstanceClasses = nil
	if r := ValidateNodeGroup(state, admissionv1.Create); r.HasErrors() {
		t.Fatalf("ValidateNodeGroup(Create) = %q, want allow missing InstanceClass", r.Error())
	}
}

func TestValidateCredentialSecretDoesNotRequirePrimaryCredentialSecret(t *testing.T) {
	t.Parallel()
	state := validState(t)
	state.CredentialSecrets = nil
	if r := ValidateCredentialSecret(state, admissionv1.Update); r.HasErrors() {
		t.Fatalf("ValidateCredentialSecret() = %q, want no primary credential requirement", r.Error())
	}
}

func validState(t *testing.T) *dvpval.State {
	t.Helper()
	return &dvpval.State{
		ModuleName:    dvpmeta.ModuleName,
		NamespaceName: dvpmeta.Namespace,
		ModuleConfig: &cpapi.ModuleConfig[*dvpsettings.ModuleConfigSettings]{
			ObjectMeta: cpapi.ObjectMeta{Name: dvpmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec[*dvpsettings.ModuleConfigSettings]{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: &dvpsettings.ModuleConfigSettings{
					Provider: dvpsettings.Provider{
						Parameters: dvpsettings.ProviderParameters{Namespace: dvpmeta.Namespace, NetworkPolicy: "Isolated"},
					},
					Storage: dvpsettings.Storage{Disabled: false},
					Nodes:   dvpsettings.Nodes{Disabled: true},
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
