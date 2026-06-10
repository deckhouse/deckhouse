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
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateInvariantsRejectsUnattachedEtcdDisk(t *testing.T) {
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
	if !result.HasErrors() {
		t.Fatalf("ValidateInvariants() expected errors")
	}
	if !strings.Contains(result.Error(), "DVPInstanceClass/orphan-dvp.spec.etcdDisk") {
		t.Fatalf("ValidateInvariants() error = %q, want orphan etcdDisk path", result.Error())
	}
}

func TestValidateInvariantsSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: new(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: new(false)},
					Nodes:   &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: new(false)},
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
	state.ModuleConfig.Spec.Settings.Storage = &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: new(false)}
	state.ModuleConfig.Spec.Settings.Nodes = &cpapi.ModuleConfigSpecSubsystemSettings{
		Enabled: new(true),
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

func TestValidateModuleConfigAllowsDisabledSubsystems(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.Settings.Storage = &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: new(false)}
	state.ModuleConfig.Spec.Settings.Nodes = &cpapi.ModuleConfigSpecSubsystemSettings{Enabled: new(false)}

	result := ValidateModuleConfig(state)
	if result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateModuleConfigIgnoresSensitiveSettings(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Spec.SetRawSettings(map[string]any{
		"provider": map[string]any{
			"parameters": map[string]any{
				"token": "must-not-fail",
			},
		},
	})

	result := ValidateModuleConfig(state)
	if result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialsIgnoresOrdinaryModuleSecrets(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = append(state.CredentialSecrets,
		cpapi.CredentialSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "validation-webhook-tls",
				Namespace: Namespace,
			},
			Type: string(corev1.SecretTypeTLS),
		},
	)

	result := ValidateCredentials(state, true)
	if result.HasErrors() {
		t.Fatalf("ValidateCredentials() unexpected errors: %s", result.Error())
	}
}

func TestValidateInvariantsNilState(t *testing.T) {
	t.Parallel()

	if result := ValidateInvariants(nil); result.HasErrors() {
		t.Fatalf("ValidateInvariants(nil) = %q, want no errors", result.Error())
	}
}

func TestValidateModuleConfigRequiredWithoutLegacyPCC(t *testing.T) {
	t.Parallel()

	state := &cpval.State{}
	result := ValidateModuleConfig(state)
	if !hasViolationCode(result, "module_config_required") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}

func TestValidateModuleConfigAllowsLegacyPCCWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := &cpval.State{LegacyProviderClusterConfig: map[string]any{"masterNodeGroup": map[string]any{}}}
	if result := ValidateModuleConfig(state); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() = %q, want no errors", result.Error())
	}
}

func TestValidateModuleConfigRejectsWrongName(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.ModuleConfig.Name = "wrong-name"

	result := ValidateModuleConfig(state)
	if !hasViolationCode(result, "invalid_module_config_name") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}

func TestValidateCredentialsIgnoresUnmanagedInvalidType(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].Type = string(corev1.SecretTypeTLS)

	result := ValidateCredentials(state, true)
	if !hasViolationCode(result, "credential_secret_required") {
		t.Fatalf("ValidateCredentials() = %q, want primary required when only unmanaged secret present", result.Error())
	}
}

func TestValidateCredentialsRequiresPrimaryWhenEnabled(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidateCredentials(state, true)
	if !hasViolationCode(result, "credential_secret_required") {
		t.Fatalf("ValidateCredentials() = %q", result.Error())
	}
}

func TestValidateCredentialsSkipsPrimaryWhenNotRequired(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	if result := ValidateCredentials(state, false); result.HasErrors() {
		t.Fatalf("ValidateCredentials() = %q, want no primary requirement errors", result.Error())
	}
}

func TestValidateCredentialsIgnoresOtherNamespace(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = append(state.CredentialSecrets, cpapi.CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: "other"},
		Type:       cpapi.CredentialsSecretType,
		StringData: cpapi.CredentialSecretStringData{AuthScheme: "invalid"},
	})

	if result := ValidateCredentials(state, true); result.HasErrors() {
		t.Fatalf("ValidateCredentials() = %q, want other namespace secret ignored", result.Error())
	}
}

func TestValidateInstanceClassDeleteEmptyName(t *testing.T) {
	t.Parallel()

	if result := ValidateInstanceClassDelete(validState(t), "", nil); result.HasErrors() {
		t.Fatalf("ValidateInstanceClassDelete() = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassDeleteInUseByNodeGroup(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassDelete(validState(t), "master-dvp", nil)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteWithStatusConsumers(t *testing.T) {
	t.Parallel()

	deleted := &cpapi.InstanceClass{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan-dvp"},
		Status:     cpapi.InstanceClassStatus{NodeGroupConsumers: []any{"worker"}},
	}
	result := ValidateInstanceClassDelete(validState(t), "", deleted)
	if !hasViolationCode(result, "instance_class_has_consumers") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteUsesDeletedClassName(t *testing.T) {
	t.Parallel()

	deleted := &cpapi.InstanceClass{
		ObjectMeta: metav1.ObjectMeta{Name: "master-dvp"},
	}
	result := ValidateInstanceClassDelete(validState(t), "", deleted)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func validState(t *testing.T) *cpval.State {
	t.Helper()

	state := &cpval.State{
		ModuleConfig: &cpapi.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ModuleName},
			Spec: cpapi.ModuleConfigSpec{
				Enabled: new(true),
				Version: 2,
				Settings: cpapi.ModuleConfigSpecSettings{
					Storage: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled:    new(true),
						Parameters: map[string]any{},
					},
					Nodes: &cpapi.ModuleConfigSpecSubsystemSettings{
						Enabled: new(false),
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
	state.ModuleConfig.Spec.SetRawSettings(map[string]any{
		"provider": map[string]any{
			"parameters": map[string]any{
				"namespace": Namespace,
			},
		},
		"storage": map[string]any{
			"enabled":    true,
			"parameters": map[string]any{},
		},
		"nodes": map[string]any{
			"enabled": false,
		},
	})

	return state
}

func rawJSONForTest(value string) *json.RawMessage {
	message := json.RawMessage(value)
	return &message
}

func validKubeconfigB64ForTest() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}
