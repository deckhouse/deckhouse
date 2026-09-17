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

	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
	zadmission "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
)

// ModuleConfigValidator reviews writes to the module's own ModuleConfig.
type ModuleConfigValidator struct {
	factory *zval.AdmissionStateBuilderFactory
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*ModuleConfigValidator)(nil)
	_ cpwebhook.Registrar       = (*ModuleConfigValidator)(nil)

	moduleConfigLog = logf.Log.WithName("module-config")
)

// NewModuleConfigValidator builds the validator for the given reviewed object kind.
func NewModuleConfigValidator(factory *zval.AdmissionStateBuilderFactory, object runtime.Object) *ModuleConfigValidator {
	return &ModuleConfigValidator{
		factory: factory,
		object:  object,
	}
}

// Register wires the validator into the webhook server.
func (v *ModuleConfigValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(v.object).
		WithValidator(v).
		Complete()
}

// ValidateCreate reviews a newly created ModuleConfig.
func (v *ModuleConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, obj)
}

// ValidateUpdate reviews the new state of an edited ModuleConfig.
func (v *ModuleConfigValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Update, newObj)
}

// ValidateDelete allows every deletion: disabling the module is the operator's call, and a
// ModuleConfig on its way out cannot be made valid by refusing to remove it.
func (v *ModuleConfigValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *ModuleConfigValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	name := objectName(obj)

	if name != zmeta.ModuleName {
		moduleConfigLog.V(2).Info("skipping validation", "reason", "not this module's ModuleConfig", "name", name)
		return nil, nil
	}

	moduleConfigLog.Info(
		"validating resource",
		"operation", operation,
		"resource", "ModuleConfig",
		"name", name,
	)

	// The reviewed object is the ModuleConfig itself, so it goes into the state directly rather
	// than being read back from the cluster — the stored copy is still the old one.
	state, err := v.factory.CreateBuilder().
		SetModuleConfig(ctx, obj).
		Build(ctx)
	if err != nil {
		moduleConfigLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		moduleConfigLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := zadmission.ValidateModuleConfig(state, operation)

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		errorViolations := result.Errors()
		warningViolations := result.Warnings()

		moduleConfigLog.Info("validation denied", "errors", len(errorViolations), "warnings", len(warningViolations))
		moduleConfigLog.V(1).Info("validation errors", "errors", violationMessages(errorViolations), "warnings", violationMessages(warningViolations))

		return warnings, admissionErr
	}

	moduleConfigLog.Info(
		"validation allowed",
		"operation", operation,
		"resource", "ModuleConfig",
		"name", name,
	)

	return warnings, nil
}
