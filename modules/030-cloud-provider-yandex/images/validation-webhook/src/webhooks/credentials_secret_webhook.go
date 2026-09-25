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
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
	ycadmission "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation/admission"
)

type CredentialSecretValidator struct {
	factory *ycval.AdmissionStateBuilderFactory
}

var (
	_ admission.Validator[*corev1.Secret] = (*CredentialSecretValidator)(nil)
	_ cpwebhook.Registrar                 = (*CredentialSecretValidator)(nil)

	credentialSecretLog = logf.Log.WithName("credential-secret")
)

func NewCredentialSecretValidator(factory *ycval.AdmissionStateBuilderFactory) *CredentialSecretValidator {
	return &CredentialSecretValidator{
		factory: factory,
	}
}

func (v *CredentialSecretValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager, &corev1.Secret{}).
		WithValidator(v).
		Complete()
}

func (v *CredentialSecretValidator) ValidateCreate(ctx context.Context, secret *corev1.Secret) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, secret)
}

func (v *CredentialSecretValidator) ValidateUpdate(ctx context.Context, oldSecret, newSecret *corev1.Secret) (admission.Warnings, error) {
	if err := validateCredentialSecretTypeChange(oldSecret, newSecret); err != nil {
		return nil, err
	}

	return v.validate(ctx, admissionv1.Update, newSecret)
}

func (v *CredentialSecretValidator) ValidateDelete(ctx context.Context, secret *corev1.Secret) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Delete, secret)
}

func (v *CredentialSecretValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	secret *corev1.Secret,
) (admission.Warnings, error) {
	namespace := secret.Namespace
	name := secret.Name

	if namespace != ycmeta.Namespace {
		credentialSecretLog.V(2).Info("skipping validation", "reason", "not module namespace", "namespace", namespace, "name", name)
		return nil, nil
	}

	if secret.Type != cpapi.CredentialsSecretType {
		credentialSecretLog.V(2).Info("skipping validation", "reason", "not managed credential secret", "name", name)
		return nil, nil
	}

	credentialSecretLog.Info(
		"validating resource",
		"operation", operation,
		"resource", "Secret",
		"name", name,
		"namespace", namespace,
	)

	builder := v.factory.CreateBuilder()
	if operation != admissionv1.Delete {
		builder = builder.SetCredentialSecret(ctx, secret)
	}

	state, err := builder.Build(ctx)
	if err != nil {
		credentialSecretLog.Error(err, "failed to build validation state", "name", name, "namespace", namespace)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		credentialSecretLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := ycadmission.ValidateCredentialSecret(state, operation)

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		errorViolations := result.Errors()
		warningViolations := result.Warnings()

		credentialSecretLog.Info("validation denied", "errors", len(errorViolations), "warnings", len(warningViolations))
		credentialSecretLog.V(1).Info("validation errors", "errors", violationMessages(errorViolations), "warnings", violationMessages(warningViolations))

		return warnings, admissionErr
	}

	credentialSecretLog.Info(
		"validation allowed",
		"operation", operation,
		"resource", "Secret",
		"name", name,
		"namespace", namespace,
	)

	return warnings, nil
}

func validateCredentialSecretTypeChange(oldSecret, newSecret *corev1.Secret) error {
	if oldSecret == nil || newSecret == nil {
		return nil
	}

	if oldSecret.Namespace != ycmeta.Namespace {
		return nil
	}

	if oldSecret.Type != cpapi.CredentialsSecretType {
		return nil
	}

	if newSecret.Type == cpapi.CredentialsSecretType {
		return nil
	}

	return invalidCredentialSecretTypeError(newSecret.Name)
}

func invalidCredentialSecretTypeError(name string) error {
	return apierrors.NewInvalid(
		corev1.SchemeGroupVersion.WithKind("Secret").GroupKind(),
		name,
		field.ErrorList{
			field.Invalid(
				field.NewPath("type"),
				cpapi.CredentialsSecretType,
				fmt.Sprintf("credential Secret type must be %q", cpapi.CredentialsSecretType),
			),
		},
	)
}
