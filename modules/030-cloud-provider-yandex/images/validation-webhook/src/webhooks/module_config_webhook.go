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

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
	ycadmission "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation/admission"
)

// ModuleConfigValidator validates the cloud-provider-yandex ModuleConfig.
type ModuleConfigValidator struct {
	factory *ycval.AdmissionStateBuilderFactory
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*ModuleConfigValidator)(nil)
	_ cpwebhook.Registrar       = (*ModuleConfigValidator)(nil)

	moduleConfigLog = logf.Log.WithName("module-config")
)

func NewModuleConfigValidator(factory *ycval.AdmissionStateBuilderFactory, object runtime.Object) *ModuleConfigValidator {
	return &ModuleConfigValidator{
		factory: factory,
		object:  object,
	}
}

func (v *ModuleConfigValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(v.object).
		WithValidator(v).
		Complete()
}

func (v *ModuleConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, obj)
}

func (v *ModuleConfigValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Update, newObj)
}

func (v *ModuleConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Delete, obj)
}

// shouldValidateModuleConfig reports whether the reviewed object is the provider ModuleConfig.
// ModuleConfig is cluster-scoped, so the name alone identifies it.
func (v *ModuleConfigValidator) shouldValidateModuleConfig(obj runtime.Object) bool {
	return objectName(obj) == ycmeta.ModuleName
}

func (v *ModuleConfigValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	name := objectName(obj)

	if !v.shouldValidateModuleConfig(obj) {
		moduleConfigLog.V(2).Info("skipping validation", "reason", "not the provider ModuleConfig", "name", name)
		return nil, nil
	}

	moduleConfigLog.Info(
		"validating resource",
		"operation", operation,
		"resource", "ModuleConfig",
		"name", name,
	)

	// ValidateNodeGroupExternalIPAddresses compares settings.nodes.parameters.externalIPAddresses
	// against the node count of every NodeGroup, so the whole set has to be in the state: the
	// reviewed object here is the ModuleConfig and carries no NodeGroups of its own.
	builder := v.factory.CreateBuilder().AddNodeGroups(ctx)
	if operation != admissionv1.Delete {
		builder = builder.SetModuleConfig(ctx, obj)
	}

	state, err := builder.Build(ctx)
	if err != nil {
		moduleConfigLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		moduleConfigLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := ycadmission.ValidateModuleConfig(state, operation)

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
