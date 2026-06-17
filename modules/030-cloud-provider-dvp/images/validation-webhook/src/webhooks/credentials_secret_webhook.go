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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpadmission "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/admission"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/meta"
)

type CredentialSecretValidator struct {
	builder *cpvaladmission.StateBuilder
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*CredentialSecretValidator)(nil)
	_ cpwebhook.Registrar       = (*CredentialSecretValidator)(nil)

	credentialSecretLog = logf.Log.WithName("credential-secret")
)

func NewCredentialSecretValidator(builder *cpvaladmission.StateBuilder, object runtime.Object) *CredentialSecretValidator {
	return &CredentialSecretValidator{
		builder: builder,
		object:  object,
	}
}

func (v *CredentialSecretValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(v.object).
		WithValidator(v).
		Complete()
}

func (v *CredentialSecretValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, obj)
}

func (v *CredentialSecretValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	if err := validateCredentialSecretTypeChange(oldObj, newObj); err != nil {
		return nil, err
	}

	return v.validate(ctx, admissionv1.Update, newObj)
}

func (v *CredentialSecretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Delete, obj)
}

func (v *CredentialSecretValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	namespace := objectNamespace(obj)
	name := objectName(obj)

	if namespace != dvpmeta.Namespace {
		credentialSecretLog.V(2).Info("skipping validation", "reason", "not module namespace", "namespace", namespace, "name", name)
		return nil, nil
	}

	if !isManagedCredentialSecretObject(obj) {
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

	secret, err := asSecret(obj)
	if err != nil {
		credentialSecretLog.Error(err, "failed to build validation state", "name", name, "namespace", namespace)
		return nil, internalBuildError(err)
	}

	state, err := v.builder.BuildForCredentialSecret(ctx, operation, cpvaladmission.SecretToCredentialSecret(secret))
	if err != nil {
		credentialSecretLog.Error(err, "failed to build validation state", "name", name, "namespace", namespace)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		credentialSecretLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := dvpadmission.ValidateCredentialSecret(state, operation)

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

func isManagedCredentialSecretObject(obj runtime.Object) bool {
	if secret, ok := obj.(*corev1.Secret); ok {
		return cpvaladmission.IsManagedCredentialSecret(secret)
	}

	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		secretType, _, _ := unstructured.NestedString(unstructuredObj.Object, "type")
		return secretType == cpapi.CredentialsSecretType
	}

	return false
}

func validateCredentialSecretTypeChange(oldObj, newObj runtime.Object) error {
	if oldObj == nil || newObj == nil {
		return nil
	}

	oldSecret, errOld := asSecret(oldObj)
	if errOld != nil {
		return internalBuildError(fmt.Errorf("decode old Secret: %w", errOld))
	}

	newSecret, errNew := asSecret(newObj)
	if errNew != nil {
		return internalBuildError(fmt.Errorf("decode new Secret: %w", errNew))
	}

	if objectNamespace(oldSecret) != dvpmeta.Namespace {
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

func asSecret(obj runtime.Object) (*corev1.Secret, error) {
	if secret, ok := obj.(*corev1.Secret); ok {
		return secret, nil
	}

	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		secret := &corev1.Secret{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, secret); err != nil {
			return nil, fmt.Errorf("convert unstructured Secret: %w", err)
		}

		return secret, nil
	}

	return nil, fmt.Errorf("expected Secret object but got %T", obj)
}
