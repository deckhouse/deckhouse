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
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var staticmachinelog = logf.Log.WithName("staticmachine-resource")

func (r *StaticMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

///+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachines,verbs=create;update,versions=v1alpha1,name=mstaticmachine.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &StaticMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StaticMachine) Default() {
	staticmachinelog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachines,verbs=update,versions=v1alpha1,name=vstaticmachine.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Validator = &StaticMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachine) ValidateCreate() (admission.Warnings, error) {
	staticmachinelog.Info("validate create", "name", r.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachine) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	staticmachinelog.Info("validate update", "name", r.Name)

	var errs field.ErrorList

	// By convention, StaticMachine.spec is immutable except for the providerID field.
	newStaticMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return nil, apierrors.NewInternalError(errors.Wrap(err, "failed to convert new StaticMachine to unstructured object"))
	}

	oldStaticMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return nil, apierrors.NewInternalError(errors.Wrap(err, "failed to convert old StaticMachine to unstructured object"))
	}

	newStaticMachineSpec := newStaticMachine["spec"].(map[string]interface{})
	oldStaticMachineSpec := oldStaticMachine["spec"].(map[string]interface{})

	// Allow changes to providerID.
	delete(oldStaticMachineSpec, "providerID")
	delete(newStaticMachineSpec, "providerID")

	if !reflect.DeepEqual(oldStaticMachineSpec, newStaticMachineSpec) {
		errs = append(errs, field.Forbidden(field.NewPath("spec"), "cannot be modified"))
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachine) ValidateDelete() (admission.Warnings, error) {
	staticmachinelog.Info("validate delete", "name", r.Name)

	return nil, nil
}
