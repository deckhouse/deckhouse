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

	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

type DVPInstanceClassValidator struct {
	builder *cpvaladmission.StateBuilder
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*DVPInstanceClassValidator)(nil)
	_ cpwebhook.Registrar       = (*DVPInstanceClassValidator)(nil)

	instanceClassLog = logf.Log.WithName("instance-class")
)

func NewDVPInstanceClassValidator(builder *cpvaladmission.StateBuilder, object runtime.Object) *DVPInstanceClassValidator {
	return &DVPInstanceClassValidator{
		builder: builder,
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
	instanceClassLog.Info(
		"validating resource",
		"operation", operation,
		"resource", dvpval.InstanceClassKind,
		"name", name,
		"namespace", objectNamespace(obj),
	)

	state, deletedClass, err := v.builder.BuildForInstanceClass(ctx, operation, obj)
	if err != nil {
		instanceClassLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		instanceClassLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := dvpval.ValidateInvariants(state)

	if operation == admissionv1.Delete {
		deleteResult := cpval.ValidateInstanceClassDelete(state, name, deletedClass)
		result.Merge(deleteResult)
	}

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		instanceClassLog.Info("validation denied", "violations", len(result.Errors()))
		return warnings, admissionErr
	}

	return warnings, nil
}
