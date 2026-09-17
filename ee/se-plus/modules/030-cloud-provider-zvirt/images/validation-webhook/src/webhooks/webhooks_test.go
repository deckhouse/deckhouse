/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package webhooks

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

func TestValidatorsAllowAValidCluster(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	ctx := context.Background()

	if _, err := NewCredentialSecretValidator(factory, &corev1.Secret{}).
		ValidateCreate(ctx, credentialSecret("admin@internal", "password")); err != nil {
		t.Errorf("credential secret: ValidateCreate() = %v, want allow", err)
	}

	if _, err := NewNodeGroupValidator(factory, &unstructured.Unstructured{}).
		ValidateCreate(ctx, nodeGroupObject("master", cpapi.NodeTypeCloudPermanent)); err != nil {
		t.Errorf("node group: ValidateCreate() = %v, want allow", err)
	}

	if _, err := NewZvirtInstanceClassValidator(factory, &unstructured.Unstructured{}).
		ValidateUpdate(ctx, nil, instanceClassObject(masterInstanceClassName, true)); err != nil {
		t.Errorf("instance class: ValidateUpdate() = %v, want allow", err)
	}
}

func TestModuleConfigValidatorAcceptsAValidConfiguration(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	obj := moduleConfigObject(zmeta.ModuleName, "https://zvirt.example.com/ovirt-engine/api", testCABundle(t), false)

	if _, err := NewModuleConfigValidator(factory, &unstructured.Unstructured{}).
		ValidateCreate(context.Background(), obj); err != nil {
		t.Fatalf("ValidateCreate() = %v, want allow", err)
	}
}

func TestModuleConfigValidatorRejectsABrokenEndpoint(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	validator := NewModuleConfigValidator(factory, &unstructured.Unstructured{})

	for _, operation := range []string{"create", "update"} {
		obj := moduleConfigObject(zmeta.ModuleName, "zvirt.example.com", "", false)

		var err error
		if operation == "create" {
			_, err = validator.ValidateCreate(context.Background(), obj)
		} else {
			_, err = validator.ValidateUpdate(context.Background(), nil, obj)
		}

		if err == nil {
			t.Fatalf("Validate%s() = nil, want the endpoint to be rejected", operation)
		}

		if !strings.Contains(err.Error(), "invalid zVirt API endpoint") {
			t.Fatalf("Validate%s() = %v, want it to name the endpoint", operation, err)
		}
	}
}

func TestModuleConfigValidatorRejectsABrokenCABundle(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	obj := moduleConfigObject(zmeta.ModuleName, "https://zvirt.example.com", "%%% not base64 %%%", false)

	_, err := NewModuleConfigValidator(factory, &unstructured.Unstructured{}).
		ValidateUpdate(context.Background(), nil, obj)
	if err == nil {
		t.Fatalf("ValidateUpdate() = nil, want the CA bundle to be rejected")
	}

	if !strings.Contains(err.Error(), "invalid CA bundle") {
		t.Fatalf("ValidateUpdate() = %v, want it to name the CA bundle", err)
	}
}

// A CA bundle together with insecure is a warning, not a denial: the cluster works, it just does
// not verify the certificate it was given.
func TestModuleConfigValidatorWarnsAboutAnIgnoredCABundle(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	obj := moduleConfigObject(zmeta.ModuleName, "https://zvirt.example.com", testCABundle(t), true)

	warnings, err := NewModuleConfigValidator(factory, &unstructured.Unstructured{}).
		ValidateUpdate(context.Background(), nil, obj)
	if err != nil {
		t.Fatalf("ValidateUpdate() = %v, want the request to be allowed", err)
	}

	if len(warnings) != 1 || !strings.Contains(warnings[0], "insecure") {
		t.Fatalf("ValidateUpdate() warnings = %v, want one about the ignored caBundle", warnings)
	}
}

// Every ModuleConfig in the cluster matches the resource rule; another module's is none of this
// webhook's business, however broken it looks.
func TestModuleConfigValidatorIgnoresOtherModules(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	obj := moduleConfigObject("cloud-provider-yandex", "not-a-url", "", false)

	if _, err := NewModuleConfigValidator(factory, &unstructured.Unstructured{}).
		ValidateCreate(context.Background(), obj); err != nil {
		t.Fatalf("ValidateCreate() = %v, want another module's config to be ignored", err)
	}
}

func TestModuleConfigValidatorAllowsDeletion(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	obj := moduleConfigObject(zmeta.ModuleName, "not-a-url", "", false)

	if _, err := NewModuleConfigValidator(factory, &unstructured.Unstructured{}).
		ValidateDelete(context.Background(), obj); err != nil {
		t.Fatalf("ValidateDelete() = %v, want deletion to be allowed", err)
	}
}

func TestCredentialSecretValidatorRejectsAMissingPassword(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	secret := credentialSecret("admin@internal", "")

	_, err := NewCredentialSecretValidator(factory, &corev1.Secret{}).
		ValidateCreate(context.Background(), secret)
	if err == nil {
		t.Fatalf("ValidateCreate() = nil, want the empty password to be rejected")
	}

	if !strings.Contains(err.Error(), "secret is required") {
		t.Fatalf("ValidateCreate() = %v, want it to name the missing secret key", err)
	}
}

// A Secret outside the module namespace is none of the webhook's business, whatever its type.
func TestCredentialSecretValidatorIgnoresForeignNamespaces(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	secret := credentialSecret("admin@internal", "")
	secret.Namespace = "default"

	if _, err := NewCredentialSecretValidator(factory, &corev1.Secret{}).
		ValidateCreate(context.Background(), secret); err != nil {
		t.Fatalf("ValidateCreate() = %v, want a Secret in another namespace to be ignored", err)
	}
}

// Likewise a Secret of an unrelated type living in the module namespace.
func TestCredentialSecretValidatorIgnoresUnmanagedTypes(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)
	secret := credentialSecret("", "")
	secret.Type = corev1.SecretTypeOpaque

	if _, err := NewCredentialSecretValidator(factory, &corev1.Secret{}).
		ValidateCreate(context.Background(), secret); err != nil {
		t.Fatalf("ValidateCreate() = %v, want an unmanaged Secret to be ignored", err)
	}
}

// Retyping a managed Secret would take it out of every rule's view while the components carry on
// reading it, so the type is pinned for as long as the Secret exists.
func TestCredentialSecretValidatorRejectsRetyping(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)

	oldSecret := credentialSecret("admin@internal", "password")
	newSecret := credentialSecret("admin@internal", "password")
	newSecret.Type = corev1.SecretTypeOpaque

	_, err := NewCredentialSecretValidator(factory, &corev1.Secret{}).
		ValidateUpdate(context.Background(), oldSecret, newSecret)
	if err == nil {
		t.Fatalf("ValidateUpdate() = nil, want the type change to be rejected")
	}

	if !strings.Contains(err.Error(), cpapi.CredentialsSecretType) {
		t.Fatalf("ValidateUpdate() = %v, want it to name the required type", err)
	}
}

func TestInstanceClassValidatorRejectsRemovingTheMasterEtcdDisk(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)

	_, err := NewZvirtInstanceClassValidator(factory, &unstructured.Unstructured{}).
		ValidateUpdate(context.Background(), nil, instanceClassObject(masterInstanceClassName, false))
	if err == nil {
		t.Fatalf("ValidateUpdate() = nil, want the missing etcd disk to be rejected")
	}

	// The shared rule has to name zVirt's own field, not the etcdDisk other providers use.
	if !strings.Contains(err.Error(), "etcdDiskSizeGb") {
		t.Fatalf("ValidateUpdate() = %v, want it to name spec.etcdDiskSizeGb", err)
	}
}

func TestInstanceClassValidatorRejectsDeletionWhileInUse(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validClusterObjects()...)

	_, err := NewZvirtInstanceClassValidator(factory, &unstructured.Unstructured{}).
		ValidateDelete(context.Background(), instanceClassObject(masterInstanceClassName, true))
	if err == nil {
		t.Fatalf("ValidateDelete() = nil, want the class to be held by NodeGroup/master")
	}
}

// A NodeGroup may legitimately be created before its class exists; the webhook reviews one write
// at a time and must not demand a complete cluster.
func TestNodeGroupValidatorAllowsAClassThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, credentialSecret("admin@internal", "password"))

	if _, err := NewNodeGroupValidator(factory, &unstructured.Unstructured{}).
		ValidateCreate(context.Background(), nodeGroupObject("master", cpapi.NodeTypeCloudPermanent)); err != nil {
		t.Fatalf("ValidateCreate() = %v, want allow", err)
	}
}

func TestShouldValidateNodeGroup(t *testing.T) {
	t.Parallel()

	validator := NewNodeGroupValidator(nil, &unstructured.Unstructured{})

	tests := []struct {
		name string
		obj  runtime.Object
		want bool
	}{
		{
			name: "a cloud-permanent NodeGroup is ours",
			obj:  nodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
			want: true,
		},
		{
			name: "a static NodeGroup with no class reference is not",
			obj:  staticNodeGroupObject("worker"),
			want: false,
		},
		{
			// Fail closed: an object the decoder cannot make sense of is handed to the rules,
			// which is where the operator gets told what is wrong with it.
			name: "an undecodable object is reviewed rather than waved through",
			obj:  malformedNodeGroupObject(),
			want: true,
		},
		{
			// A NodeGroup of another provider is skipped even though it decodes cleanly.
			name: "a static NodeGroup pointing at a foreign class is not ours",
			obj:  foreignClassNodeGroupObject(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := validator.shouldValidateNodeGroup(tt.obj); got != tt.want {
				t.Errorf("shouldValidateNodeGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViolationFieldPathDropsTheResourceSegment(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                                     "spec",
		"NodeGroup/master.spec.cloudInstances": "master.spec.cloudInstances",
		"ModuleConfig.spec.settings.provider":  "ModuleConfig.spec.settings.provider",
	}

	for path, want := range tests {
		if got := violationFieldPath(path).String(); got != want {
			t.Errorf("violationFieldPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestResultToAdmissionSeparatesWarningsFromErrors(t *testing.T) {
	t.Parallel()

	result := cpvalapi.Result{}
	result.AddWarning("ModuleConfig.spec.settings.provider.parameters.caBundle", "ignored", nil, "caBundle is ignored")

	warnings, err := resultToAdmission(result)
	if err != nil {
		t.Fatalf("resultToAdmission() error = %v, want a warning not to deny the request", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("resultToAdmission() warnings = %v, want exactly one", warnings)
	}

	result.AddError("Secret/d8-credentials", "required", nil, "credential Secret is required")
	if _, err := resultToAdmission(result); err == nil {
		t.Fatalf("resultToAdmission() = nil, want an error once a violation is an error")
	}
}
