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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	cpwebhookstate "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook/state"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

type DVPInstanceClassValidator struct {
	builder cpwebhookstate.Builder
	object  runtime.Object
}

var (
	_ admission.CustomValidator = (*DVPInstanceClassValidator)(nil)
	_ cpwebhook.Registrar       = (*DVPInstanceClassValidator)(nil)
)

func NewDVPInstanceClassValidator(builder cpwebhookstate.Builder, object runtime.Object) *DVPInstanceClassValidator {
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
	state, deletedClass, err := v.builder.BuildForInstanceClass(ctx, operation, obj)
	if err != nil {
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		return nil, nil
	}

	result := validateAdmissionState(state)

	if operation == admissionv1.Delete {
		deleteResult := dvpval.ValidateInstanceClassDelete(state, objectName(obj), deletedClass)
		result.Merge(deleteResult)
	}

	return resultToAdmission(result)
}
