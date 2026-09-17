/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package webhooks

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
	zadmission "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation/admission"
	cpadmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
)

type ZvirtInstanceClassValidator struct {
	factory *zval.AdmissionStateBuilderFactory
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*ZvirtInstanceClassValidator)(nil)
	_ cpwebhook.Registrar       = (*ZvirtInstanceClassValidator)(nil)

	instanceClassLog = logf.Log.WithName("instance-class")
)

func NewZvirtInstanceClassValidator(factory *zval.AdmissionStateBuilderFactory, object runtime.Object) *ZvirtInstanceClassValidator {
	return &ZvirtInstanceClassValidator{
		factory: factory,
		object:  object,
	}
}

func (v *ZvirtInstanceClassValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(v.object).
		WithValidator(v).
		Complete()
}

func (v *ZvirtInstanceClassValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, obj)
}

func (v *ZvirtInstanceClassValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Update, newObj)
}

func (v *ZvirtInstanceClassValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Delete, obj)
}

func (v *ZvirtInstanceClassValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	name := objectName(obj)
	namespace := objectNamespace(obj)

	instanceClassLog.Info(
		"validating resource",
		"operation", operation,
		"resource", zicv1.GroupVersionKind.Kind,
		"name", name,
		"namespace", namespace,
	)

	builder := v.factory.CreateBuilder().AddAssociatedNodeGroups(ctx, name)
	if operation != admissionv1.Delete {
		builder = builder.SetInstanceClass(ctx, obj)
	}

	state, err := builder.Build(ctx)
	if err != nil {
		instanceClassLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		instanceClassLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	// On Delete the reviewed class is passed to the deletion rule instead of the state: it is
	// going away, so it must not look like an existing class to the other rules.
	var deletedClass *zicv1.ZvirtInstanceClass
	if operation == admissionv1.Delete {
		deletedClass, err = cpadmission.DecodeInstanceClassObject[*zicv1.ZvirtInstanceClass](obj)
		if err != nil {
			instanceClassLog.Error(err, "failed to decode instance class", "name", name)
			return nil, internalBuildError(err)
		}
	}

	result := zadmission.ValidateInstanceClass(state, operation, deletedClass)

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		errorViolations := result.Errors()
		warningViolations := result.Warnings()

		instanceClassLog.Info("validation denied", "errors", len(errorViolations), "warnings", len(warningViolations))
		instanceClassLog.V(1).Info("validation errors", "errors", violationMessages(errorViolations), "warnings", violationMessages(warningViolations))

		return warnings, admissionErr
	}

	instanceClassLog.Info(
		"validation allowed",
		"operation", operation,
		"resource", zicv1.GroupVersionKind.Kind,
		"name", name,
		"namespace", namespace,
	)

	return warnings, nil
}
