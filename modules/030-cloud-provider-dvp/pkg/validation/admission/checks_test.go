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

	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
)

func hasViolationCode(result cpval.Result, code string) bool {
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
		state.InstanceClasses, cpapi.InstanceClass{
			TypeMeta:   cpapi.TypeMeta{Kind: dvpmeta.InstanceClassKind},
			ObjectMeta: cpapi.ObjectMeta{Name: "orphan-dvp"},
			Spec: cpapi.InstanceClassSpec{
				EtcdDisk: map[string]any{},
			},
		},
	)

	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClass() = %q, want unattached etcdDisk allowed", result.Error())
	}
}

func TestValidateAdmissionSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		ModuleName:        dvpmeta.ModuleName,
		NamespaceName:     dvpmeta.Namespace,
		InstanceClassKind: dvpmeta.InstanceClassKind,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: cpapi.ObjectMeta{Name: dvpmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{Disabled: ptr.To(true)},
					Nodes:   &cpapi.ModuleConfigSpecSubsystemSettings{Disabled: ptr.To(true)},
				},
			},
		},
		MigrationStatus: cpapi.MigrationStatus{
			LegacyPCCPresent: true,
			MigrationPending: true,
		},
	}

	for name, validate := range map[string]func(*cpval.State) cpval.Result{
		"ValidateCredentialSecret": func(state *cpval.State) cpval.Result {
			return ValidateCredentialSecret(state, admissionv1.Update)
		},
		"ValidateInstanceClass": func(state *cpval.State) cpval.Result {
			return ValidateInstanceClass(state, admissionv1.Update, nil)
		},
		"ValidateNodeGroup": func(state *cpval.State) cpval.Result {
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
	state.InstanceClasses[0].Spec.EtcdDisk = nil

	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if !hasViolationCode(result, "master_etcd_disk_required") {
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

func validState(t *testing.T) *cpval.State {
	t.Helper()

	state := &cpval.State{
		ModuleName:        dvpmeta.ModuleName,
		NamespaceName:     dvpmeta.Namespace,
		InstanceClassKind: dvpmeta.InstanceClassKind,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: cpapi.ObjectMeta{Name: dvpmeta.ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Provider: &cpapi.ModuleConfigSpecProviderSettings{
						Parameters: map[string]any{
							"namespace": dvpmeta.Namespace,
						},
					},
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{
						Disabled:   ptr.To(false),
						Parameters: map[string]any{},
					},
					Nodes: &cpapi.ModuleConfigSpecSubsystemSettings{
						Disabled: ptr.To(true),
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{
					Name:      cpapi.CredentialSecretName,
					Namespace: dvpmeta.Namespace,
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
							Kind: dvpmeta.InstanceClassKind,
							Name: "master-dvp",
						},
					},
				},
			},
		},
		InstanceClasses: []cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: dvpmeta.InstanceClassKind},
				ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
		},
	}
	return state
}

func validKubeconfigB64ForTest() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}
