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
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func hasViolationCode(result cpval.Result, code string) bool {
	for _, violation := range result.Errors {
		if violation.Code == code {
			return true
		}
	}
	return false
}

func TestValidatePreflightNilState(t *testing.T) {
	t.Parallel()

	if result := ValidatePreflight(nil); result.HasErrors() {
		t.Fatalf("ValidatePreflight(nil) = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := &cpval.State{
		MigrationStatus: cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true},
	}
	if result := ValidatePreflight(state); result.HasErrors() {
		t.Fatalf("ValidatePreflight() during migration = %q, want no errors", result.Error())
	}
}

func TestValidatePreflightRequiresCredentialSecret(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_credential_secret_required") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidCredentialSecretType(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.CredentialSecrets[0].Type = string(corev1.SecretTypeTLS)

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_invalid_credential_secret_type") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterNodeGroup(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_node_group_required") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterClassReference(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_class_reference_required") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRejectsInvalidInstanceClassKind(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_invalid_instance_class_kind") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresInstanceClassName(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = "  "

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_instance_class_name_required") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresExistingInstanceClass(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_instance_class_not_found") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}

func TestValidatePreflightRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.InstanceClasses[0].Spec.EtcdDisk = nil

	result := ValidatePreflight(state)
	if !hasViolationCode(result, "preflight_master_etcd_disk_required") {
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

func TestFindHelpers(t *testing.T) {
	t.Parallel()

	secret := cpapi.CredentialSecret{ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: Namespace}}
	if _, ok := findCredentialSecret([]cpapi.CredentialSecret{secret}, cpapi.CredentialSecretName); !ok {
		t.Fatal("findCredentialSecret() = false, want true")
	}
	if _, ok := findCredentialSecret([]cpapi.CredentialSecret{secret}, "missing"); ok {
		t.Fatal("findCredentialSecret() = true, want false")
	}

	nodeGroup := cpapi.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "master"}}
	if _, ok := findNodeGroup([]cpapi.NodeGroup{nodeGroup}, "master"); !ok {
		t.Fatal("findNodeGroup() = false, want true")
	}

	class := cpapi.InstanceClass{ObjectMeta: metav1.ObjectMeta{Name: "master-dvp"}}
	if _, ok := findInstanceClass([]cpapi.InstanceClass{class}, "master-dvp"); !ok {
		t.Fatal("findInstanceClass() = false, want true")
	}
}

func TestFindCredentialSecretMatchesEmptyNamespace(t *testing.T) {
	t.Parallel()

	secret := cpapi.CredentialSecret{ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName}}
	if _, ok := findCredentialSecret([]cpapi.CredentialSecret{secret}, cpapi.CredentialSecretName); !ok {
		t.Fatal("findCredentialSecret() = false for empty namespace")
	}
}

func TestValidatePreflightInvalidKindStillChecksNameWhenPresent(t *testing.T) {
	t.Parallel()

	state := validState(t)
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "WrongKind"
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Name = ""

	result := ValidatePreflight(state)
	if !strings.Contains(result.Error(), "preflight_master_instance_class_name_required") &&
		!hasViolationCode(result, "preflight_master_instance_class_name_required") {
		t.Fatalf("ValidatePreflight() = %q", result.Error())
	}
}
