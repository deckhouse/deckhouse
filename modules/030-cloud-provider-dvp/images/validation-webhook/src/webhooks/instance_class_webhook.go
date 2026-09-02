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

	cpadmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
	dvpadmission "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/admission"
)

type DVPInstanceClassValidator struct {
	factory *dvpval.AdmissionStateBuilderFactory
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*DVPInstanceClassValidator)(nil)
	_ cpwebhook.Registrar       = (*DVPInstanceClassValidator)(nil)

	instanceClassLog = logf.Log.WithName("instance-class")
)

func NewDVPInstanceClassValidator(factory *dvpval.AdmissionStateBuilderFactory, object runtime.Object) *DVPInstanceClassValidator {
	return &DVPInstanceClassValidator{
		factory: factory,
		object:  object,
	}
}

func (v *DVPInstanceClassValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager).
		For(v.object).
		WithValidator(v).
		Complete()
}

func (v *DVPInstanceClassValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Create, obj)
}

func (v *DVPInstanceClassValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Update, newObj)
}

func (v *DVPInstanceClassValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, admissionv1.Delete, obj)
}

func (v *DVPInstanceClassValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (admission.Warnings, error) {
	name := objectName(obj)
	namespace := objectNamespace(obj)

	instanceClassLog.Info(
		"validating resource",
		"operation", operation,
		"resource", dvpicv1alpha1.GroupVersionKind.Kind,
		"name", name,
		"namespace", namespace,
	)

	// The consumers are needed on every operation, Delete included: that is exactly what
	// ValidateInstanceClassDeletion reports on. Only the reviewed class itself is left out of
	// the state on Delete — it is going away.
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
	var deletedClass *dvpicv1alpha1.DVPInstanceClass
	if operation == admissionv1.Delete {
		deletedClass, err = cpadmission.DecodeInstanceClassObject[*dvpicv1alpha1.DVPInstanceClass](obj)
		if err != nil {
			instanceClassLog.Error(err, "failed to decode instance class", "name", name)
			return nil, internalBuildError(err)
		}
	}

	result := dvpadmission.ValidateInstanceClass(state, operation, deletedClass)

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
		"resource", dvpicv1alpha1.GroupVersionKind.Kind,
		"name", name,
		"namespace", namespace,
	)

	return warnings, nil
}
