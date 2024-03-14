/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var staticinstancelog = logf.Log.WithName("staticinstance-resource")

func (r *StaticInstance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

///+kubebuilder:webhook:path=/mutate-deckhouse-io-v1alpha1-staticinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=staticinstances,verbs=create;update,versions=v1alpha1,name=mstaticinstance.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &StaticInstance{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StaticInstance) Default() {
	staticinstancelog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-deckhouse-io-v1alpha1-staticinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=staticinstances,verbs=update;delete,versions=v1alpha1,name=vstaticinstance.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Validator = &StaticInstance{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstance) ValidateCreate() (admission.Warnings, error) {
	staticinstancelog.Info("validate create", "name", r.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstance) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	staticinstancelog.Info("validate update", "name", r.Name)

	oldStaticInstance := old.(*StaticInstance)
	if oldStaticInstance.Spec.Address != r.Spec.Address {
		return nil, field.Forbidden(field.NewPath("spec", "address"), "StaticInstance address is immutable")
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstance) ValidateDelete() (admission.Warnings, error) {
	staticinstancelog.Info("validate delete", "name", r.Name)

	if r.Status.CurrentStatus == nil || r.Status.CurrentStatus.Phase != StaticInstanceStatusCurrentStatusPhasePending {
		return nil, apierrors.NewForbidden(schema.GroupResource{
			Group:    r.GroupVersionKind().Group,
			Resource: "staticinstances",
		}, r.Name, errors.New(`if you need to delete a StaticInstance that is not pending, you need to add the label '"node.deckhouse.io/allow-bootstrap": "false"' to the StaticInstance, after which you need to wait until the StaticInstance status becomes 'Pending'. Do not forget to decrease the 'NodeGroup.spec.staticInstances.count' field by 1, if needed`))
	}

	return nil, nil
}
