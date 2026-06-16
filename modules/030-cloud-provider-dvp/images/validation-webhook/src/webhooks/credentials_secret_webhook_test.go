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

package webhooks

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

func TestCredentialSecretValidatorWithFakeClientValidateCreate(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	_, err := validator.ValidateCreate(context.Background(), dvpCredentialSecret(validWebhookKubeconfigB64()))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	secret := dvpCredentialSecret(validWebhookKubeconfigB64())
	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsCredentialTypeChange(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	oldSecret := dvpCredentialSecret(validWebhookKubeconfigB64())
	newSecret := oldSecret.DeepCopy()
	newSecret.Type = corev1.SecretTypeTLS

	_, err := validator.ValidateUpdate(context.Background(), oldSecret, newSecret)
	if err == nil || !strings.Contains(err.Error(), "credential Secret type must be") {
		t.Fatalf("ValidateUpdate() error = %v, want type change denial", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsInvalidAuthScheme(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	secret := dvpCredentialSecret("updated-token")
	secret.StringData[cpapi.CredentialSecretAuthSchemeKey] = string(cpapi.AuthSchemeAPIToken)

	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("ValidateUpdate() error = %v, want auth scheme denial", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientValidateDelete(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	_, err := validator.ValidateDelete(context.Background(), dvpCredentialSecret("token"))
	if err != nil {
		t.Fatalf("ValidateDelete() error = %v, want allow without preflight requirements", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientSkipsMigration(t *testing.T) {
	t.Parallel()

	objects := validDVPClusterObjects()
	objects = append(objects, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.MigrationConfigMapName,
			Namespace: dvpval.Namespace,
		},
	})
	builder := newWebhookAdmissionStateBuilder(t, objects...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	secret := dvpCredentialSecret("invalid")
	secret.StringData[cpapi.CredentialSecretAuthSchemeKey] = string(cpapi.AuthSchemeAPIToken)

	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err != nil {
		t.Fatalf("ValidateUpdate() during migration error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsCreateWithWrongType(t *testing.T) {
	t.Parallel()

	builder := newWebhookAdmissionStateBuilder(t, validDVPClusterObjects()...)
	validator := NewCredentialSecretValidator(builder, &corev1.Secret{})

	secret := dvpCredentialSecret(validWebhookKubeconfigB64())
	secret.Type = corev1.SecretTypeTLS

	_, err := validator.ValidateCreate(context.Background(), secret)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow for non-credential Secret type", err)
	}
}

func TestValidateCredentialSecretTypeChange(t *testing.T) {
	t.Parallel()

	oldSecret := dvpCredentialSecret(validWebhookKubeconfigB64())
	newSecret := oldSecret.DeepCopy()
	newSecret.Type = corev1.SecretTypeTLS

	err := validateCredentialSecretTypeChange(oldSecret, newSecret)
	if err == nil || !strings.Contains(err.Error(), "credential Secret type must be") {
		t.Fatalf("validateCredentialSecretTypeChange() error = %v, want type change denial", err)
	}

	if err := validateCredentialSecretTypeChange(oldSecret, oldSecret); err != nil {
		t.Fatalf("validateCredentialSecretTypeChange(same type) error = %v, want nil", err)
	}
}

func TestValidateCredentialSecretTypeChangeFailsClosedOnDecodeError(t *testing.T) {
	t.Parallel()

	oldSecret := dvpCredentialSecret(validWebhookKubeconfigB64())
	invalidNew := &unstructured.Unstructured{Object: map[string]any{"metadata": "invalid"}}

	err := validateCredentialSecretTypeChange(oldSecret, invalidNew)
	if err == nil || !strings.Contains(err.Error(), "build validation state") {
		t.Fatalf("validateCredentialSecretTypeChange(decode error) error = %v, want internal error", err)
	}
}

func TestRejectManagedCredentialSecretWrongTypeFailsClosedOnDecodeError(t *testing.T) {
	t.Parallel()

	if _, err := asSecret(&metav1.Status{}); err == nil {
		t.Fatal("asSecret(Status) error = nil, want type error")
	}
}

func TestIsManagedCredentialSecretObject(t *testing.T) {
	t.Parallel()

	primary := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName},
		Type:       cpapi.CredentialsSecretType,
	}
	if !isManagedCredentialSecretObject(primary) {
		t.Fatal("isManagedCredentialSecretObject(primary) = false, want true")
	}

	unstructuredSecret := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "extra-credentials"},
		"type":     cpapi.CredentialsSecretType,
	}}
	if !isManagedCredentialSecretObject(unstructuredSecret) {
		t.Fatal("isManagedCredentialSecretObject(unstructured) = false, want true")
	}

	if isManagedCredentialSecretObject(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Type: corev1.SecretTypeTLS}) {
		t.Fatal("isManagedCredentialSecretObject(tls) = true, want false")
	}
}

func TestAsSecret(t *testing.T) {
	t.Parallel()

	typed := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}}
	got, err := asSecret(typed)
	if err != nil || got.Name != "s" {
		t.Fatalf("asSecret(typed) = (%#v, %v)", got, err)
	}

	unstructuredSecret := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "from-unstructured"},
	}}
	got, err = asSecret(unstructuredSecret)
	if err != nil || got.Name != "from-unstructured" {
		t.Fatalf("asSecret(unstructured) = (%#v, %v)", got, err)
	}

	if _, err := asSecret(&unstructured.Unstructured{Object: map[string]any{"metadata": "invalid"}}); err == nil {
		t.Fatal("asSecret(invalid) error = nil, want conversion error")
	}

	if _, err := asSecret(&metav1.Status{}); err == nil {
		t.Fatal("asSecret(Status) error = nil, want type error")
	}
}
