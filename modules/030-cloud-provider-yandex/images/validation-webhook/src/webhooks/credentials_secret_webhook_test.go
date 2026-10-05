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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

func TestCredentialSecretValidatorWithFakeClientValidateCreate(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	_, err := validator.ValidateCreate(context.Background(), yandexCredentialSecret(validWebhookServiceAccountJSON()))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	secret := yandexCredentialSecret(validWebhookServiceAccountJSON())
	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsCredentialTypeChange(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	oldSecret := yandexCredentialSecret(validWebhookServiceAccountJSON())
	newSecret := oldSecret.DeepCopy()
	newSecret.Type = corev1.SecretTypeTLS

	_, err := validator.ValidateUpdate(context.Background(), oldSecret, newSecret)
	if err == nil || !strings.Contains(err.Error(), "credential Secret type must be") {
		t.Fatalf("ValidateUpdate() error = %v, want type change denial", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsInvalidAuthScheme(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	secret := yandexCredentialSecret("updated-token")
	// kubeconfig is a DVP auth scheme: Yandex accepts only serviceAccount and apiToken.
	secret.StringData[cpapi.CredentialSecretAuthSchemeKey] = string(cpapi.AuthSchemeKubeconfig)

	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("ValidateUpdate() error = %v, want auth scheme denial", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientValidateDelete(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	_, err := validator.ValidateDelete(context.Background(), yandexCredentialSecret("token"))
	if err != nil {
		t.Fatalf("ValidateDelete() error = %v, want allow without preflight requirements", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientSkipsMigration(t *testing.T) {
	t.Parallel()

	objects := validYandexClusterObjects()
	objects = append(objects, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.MigrationConfigMapName,
			Namespace: ycmeta.Namespace,
		},
	})
	factory := newWebhookAdmissionStateBuilderFactory(t, objects...)
	validator := NewCredentialSecretValidator(factory)

	secret := yandexCredentialSecret("invalid")
	// kubeconfig is a DVP auth scheme: Yandex accepts only serviceAccount and apiToken.
	secret.StringData[cpapi.CredentialSecretAuthSchemeKey] = string(cpapi.AuthSchemeKubeconfig)

	_, err := validator.ValidateUpdate(context.Background(), nil, secret)
	if err != nil {
		t.Fatalf("ValidateUpdate() during migration error = %v, want allow", err)
	}
}

func TestCredentialSecretValidatorWithFakeClientRejectsCreateWithWrongType(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewCredentialSecretValidator(factory)

	secret := yandexCredentialSecret(validWebhookServiceAccountJSON())
	secret.Type = corev1.SecretTypeTLS

	_, err := validator.ValidateCreate(context.Background(), secret)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow for non-credential Secret type", err)
	}
}

func TestValidateCredentialSecretTypeChange(t *testing.T) {
	t.Parallel()

	oldSecret := yandexCredentialSecret(validWebhookServiceAccountJSON())
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
