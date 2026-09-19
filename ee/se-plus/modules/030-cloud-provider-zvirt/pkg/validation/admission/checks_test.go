/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package admission

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
)

const masterInstanceClassName = "master-fc613b4dfd67"

func hasViolationCode(result cpvalapi.Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}

	return false
}

func TestValidateNilStateIsAnInternalError(t *testing.T) {
	t.Parallel()

	results := map[string]cpvalapi.Result{
		"credential secret": ValidateCredentialSecret(nil, admissionv1.Create),
		"module config":     ValidateModuleConfig(nil, admissionv1.Create),
		"instance class":    ValidateInstanceClass(nil, admissionv1.Create, nil),
		"node group":        ValidateNodeGroup(nil, admissionv1.Create),
	}

	for name, result := range results {
		if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
			t.Errorf("%s: got %q, want %s", name, result.Error(), cpvalapi.CodeInternalStateNil)
		}
	}
}

// Admission must let a half-migrated cluster through: the new model is not complete yet, and
// denying writes would leave the operator unable to finish the migration.
func TestValidateSkipsPendingMigration(t *testing.T) {
	t.Parallel()

	state := validState()
	state.MigrationStatus = cpapi.MigrationStatus{MigrationPending: true, LegacyPCCPresent: true}
	// Something that would certainly be rejected outside a migration.
	state.CredentialSecrets[0].StringData.Identity = ""
	state.InstanceClasses[0].Spec.EtcdDiskSizeGb = nil
	state.ModuleConfig = moduleConfig("zvirt.example.com", "")

	results := map[string]cpvalapi.Result{
		"credential secret": ValidateCredentialSecret(state, admissionv1.Create),
		"module config":     ValidateModuleConfig(state, admissionv1.Update),
		"instance class":    ValidateInstanceClass(state, admissionv1.Update, nil),
		"node group":        ValidateNodeGroup(state, admissionv1.Update),
	}

	for name, result := range results {
		if result.HasErrors() {
			t.Errorf("%s: got %q, want no errors during migration", name, result.Error())
		}
	}
}

func TestValidateModuleConfigChecksTheProviderConnection(t *testing.T) {
	t.Parallel()

	for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
		state := validState()
		state.ModuleConfig = moduleConfig("zvirt.example.com", "")

		result := ValidateModuleConfig(state, operation)
		if !hasViolationCode(result, zval.CodeProviderServerInvalid) {
			t.Errorf("ValidateModuleConfig(%s) = %q, want %s", operation, result.Error(), zval.CodeProviderServerInvalid)
		}
	}
}

// Deleting the ModuleConfig disables the module, which is the operator's call.
func TestValidateModuleConfigIgnoresDelete(t *testing.T) {
	t.Parallel()

	state := validState()
	state.ModuleConfig = moduleConfig("zvirt.example.com", "")

	if result := ValidateModuleConfig(state, admissionv1.Delete); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig(Delete) = %q, want no errors", result.Error())
	}
}

func moduleConfig(server, caBundle string) *cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings] {
	return &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{
		ObjectMeta: cpapi.ObjectMeta{Name: zmeta.ModuleName},
		Spec: cpapi.ModuleConfigSpec[*zsettingsv2.ModuleConfigSettings]{
			Version: 2,
			Settings: &zsettingsv2.ModuleConfigSettings{
				Provider: zsettingsv2.Provider{
					Parameters: zsettingsv2.ProviderParameters{Server: server, CABundle: caBundle},
				},
			},
		},
	}
}

func TestValidateCredentialSecretChecksContent(t *testing.T) {
	t.Parallel()

	state := validState()
	state.CredentialSecrets[0].StringData.Secret = ""

	result := ValidateCredentialSecret(state, admissionv1.Create)
	if !hasViolationCode(result, cpval.CodeCredentialSecretKeyRequired) {
		t.Fatalf("ValidateCredentialSecret() = %q, want %s", result.Error(), cpval.CodeCredentialSecretKeyRequired)
	}
}

// Deleting a Secret is not validated: the rules describe what a usable configuration looks like,
// and a Secret on its way out cannot be fixed by denying its removal.
func TestValidateCredentialSecretIgnoresDelete(t *testing.T) {
	t.Parallel()

	state := validState()
	state.CredentialSecrets[0].StringData.Secret = ""

	if result := ValidateCredentialSecret(state, admissionv1.Delete); result.HasErrors() {
		t.Fatalf("ValidateCredentialSecret(Delete) = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassRequiresMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	state := validState()
	state.InstanceClasses[0].Spec.EtcdDiskSizeGb = nil

	result := ValidateInstanceClass(state, admissionv1.Update, nil)
	if !hasViolationCode(result, cpval.CodeMasterEtcdDiskRequired) {
		t.Fatalf("ValidateInstanceClass() = %q, want %s", result.Error(), cpval.CodeMasterEtcdDiskRequired)
	}
}

// A class a NodeGroup still points at must not disappear from under it.
func TestValidateInstanceClassBlocksDeletionWhileInUse(t *testing.T) {
	t.Parallel()

	state := validState()
	deleted := state.InstanceClasses[0]
	deleted.Status.NodeGroupConsumers = []string{"master"}
	// On Delete the reviewed class is passed separately, never left in the state.
	state.InstanceClasses = nil

	result := ValidateInstanceClass(state, admissionv1.Delete, deleted)
	if !result.HasErrors() {
		t.Fatalf("ValidateInstanceClass(Delete) = no errors, want the class to be held by its consumers")
	}
}

func TestValidateNodeGroupRejectsAForeignInstanceClassKind(t *testing.T) {
	t.Parallel()

	state := validState()
	state.NodeGroups[0].Spec.CloudInstances.ClassReference.Kind = "YandexInstanceClass"

	result := ValidateNodeGroup(state, admissionv1.Create)
	if !hasViolationCode(result, cpval.CodeNodeGroupInvalidInstanceClassKind) {
		t.Fatalf("ValidateNodeGroup() = %q, want %s", result.Error(), cpval.CodeNodeGroupInvalidInstanceClassKind)
	}
}

// Admission deliberately does not require the referenced class to exist yet: NodeGroup and
// InstanceClass are separate resources and either order of creation has to work. Preflight, which
// judges a finished configuration rather than a single write, does require it.
func TestValidateNodeGroupAllowsAnInstanceClassThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	state := validState()
	state.InstanceClasses = nil

	if result := ValidateNodeGroup(state, admissionv1.Create); result.HasErrors() {
		t.Fatalf("ValidateNodeGroup() = %q, want no errors", result.Error())
	}
}

// Unlike preflight, admission must not demand a master NodeGroup: a single NodeGroup write is
// reviewed against a cluster that may legitimately not have one yet.
func TestValidateNodeGroupDoesNotRequireAMaster(t *testing.T) {
	t.Parallel()

	state := validState()
	state.NodeGroups = nil

	if result := ValidateNodeGroup(state, admissionv1.Create); result.HasErrors() {
		t.Fatalf("ValidateNodeGroup() = %q, want no errors", result.Error())
	}
}

func validState() *zval.State {
	class := &zicv1.ZvirtInstanceClass{}
	class.Kind = zicv1.ZvirtInstanceClassKind
	class.Name = masterInstanceClassName
	class.Spec = zicv1.InstanceClassSpec{
		NumCPUs:        4,
		Memory:         8192,
		Template:       "debian-bookworm",
		VNICProfileID:  "0000-1111",
		RootDiskSizeGb: 50,
		EtcdDiskSizeGb: ptr.To(10),
	}

	return &zval.State{
		ModuleName:    zmeta.ModuleName,
		NamespaceName: zmeta.Namespace,
		CredentialSecrets: []cpapi.CredentialSecret{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: zmeta.Namespace},
				Type:       cpapi.CredentialsSecretType,
				StringData: cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeUserPassword,
					Identity:   "admin@internal",
					Secret:     "password",
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
							Kind: zicv1.ZvirtInstanceClassKind,
							Name: masterInstanceClassName,
						},
					},
				},
			},
		},
		InstanceClasses: []*zicv1.ZvirtInstanceClass{class},
	}
}
