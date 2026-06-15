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

	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

type ModuleConfigValidator struct {
	builder *cpvaladmission.StateBuilder
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*ModuleConfigValidator)(nil)
	_ cpwebhook.Registrar       = (*ModuleConfigValidator)(nil)

	moduleConfigLog = logf.Log.WithName("module-config")
)

func NewModuleConfigValidator(builder *cpvaladmission.StateBuilder, object runtime.Object) *ModuleConfigValidator {
	return &ModuleConfigValidator{
		builder: builder,
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

func (v *ModuleConfigValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *ModuleConfigValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	name := objectName(obj)
	if name != dvpval.ModuleName {
		moduleConfigLog.V(2).Info("skipping validation", "reason", "not cloud-provider module", "name", name)
		return nil, nil
	}

	moduleConfigLog.Info(
		"validating resource",
		"operation", operation,
		"resource", "ModuleConfig",
		"name", name,
		"namespace", objectNamespace(obj),
	)

	state, err := v.builder.BuildForModuleConfig(ctx, operation, obj)
	if err != nil {
		moduleConfigLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		moduleConfigLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := dvpval.ValidateInvariants(state)

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		moduleConfigLog.Info("validation denied", "violations", len(result.Errors()))
		return warnings, admissionErr
	}

	return warnings, nil
}
