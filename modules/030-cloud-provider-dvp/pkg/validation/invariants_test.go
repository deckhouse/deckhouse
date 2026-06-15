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
	"encoding/json"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestValidateInvariantsAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = append(state.InstanceClasses, cpapi.InstanceClass{
		TypeMeta:   metav1.TypeMeta{Kind: InstanceClassKind},
		ObjectMeta: metav1.ObjectMeta{Name: "orphan-dvp"},
		Spec: cpapi.InstanceClassSpec{
			EtcdDisk: rawJSONForTest("{}"),
		},
	})

	result := ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInvariants() = %q, want unattached etcdDisk allowed", result.Error())
	}
}

func TestValidateInvariantsSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		ModuleName:      ModuleName,
		NamespaceName:   Namespace,
		InstanceClassKind: InstanceClassKind,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: ptr.To(false)},
					Nodes:   &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: ptr.To(false)},
				},
			},
		},
		MigrationStatus: cpapi.MigrationStatus{
			LegacyPCCPresent: true,
			MigrationPending: true,
		},
	}

	result := ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInvariants() during migration = %q, want no errors", result.Error())
	}
}

func TestValidateInvariantsAllowsWorkerWithoutClassReference(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = append(state.NodeGroups, cpapi.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
		},
	})

	result := ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInvariants() unexpected errors: %s", result.Error())
	}
}

func TestValidateInvariantsIgnoresNodeParameterFields(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.Settings.Storage = &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: ptr.To(false)}
	state.ModuleConfig.Spec.Settings.Nodes = &cpapi.ModuleConfigSpecSubsystemSettings{
		Enabled: ptr.To(true),
		Parameters: map[string]any{
			"layout":       "UnsupportedLayout",
			"sshPublicKey": "",
			"ipAddresses": map[string][]string{
				"missing-node-group": {"not-an-ip"},
			},
		},
	}

	result := ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInvariants() unexpected errors: %s", result.Error())
	}
}

func TestValidateInvariantsNilState(t *testing.T) {
	t.Parallel()

	result := ValidateInvariants(nil)
	if !hasViolationCode(result, cpval.CodeInternalStateNil) {
		t.Fatalf("ValidateInvariants(nil) = %q, want %s", result.Error(), cpval.CodeInternalStateNil)
	}
}

func TestValidateInvariantsDoesNotRequirePrimaryCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidateInvariants(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInvariants() = %q, want no primary credential requirement", result.Error())
	}
}

func validState(t *testing.T) *cpval.State {
	t.Helper()

	state := &cpval.State{
		ModuleName:                   ModuleName,
		NamespaceName:                Namespace,
		InstanceClassKind:            InstanceClassKind,
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: ptr.To(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Provider: &cpapi.ModuleConfigSpecProviderSettings{
						Parameters: map[string]any{
							"namespace": Namespace,
						},
					},
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled:    ptr.To(true),
						Parameters: map[string]any{},
					},
					Nodes: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cpapi.CredentialSecretName,
					Namespace: Namespace,
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
				ObjectMeta: metav1.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: InstanceClassKind,
							Name: "master-dvp",
						},
					},
				},
			},
		},
		InstanceClasses: []cpapi.InstanceClass{
			{
				TypeMeta:   metav1.TypeMeta{Kind: InstanceClassKind},
				ObjectMeta: metav1.ObjectMeta{Name: "master-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: rawJSONForTest("{}"),
				},
			},
		},
	}
	return state
}

func rawJSONForTest(value string) *json.RawMessage {
	message := json.RawMessage(value)
	return &message
}

func validKubeconfigB64ForTest() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}
