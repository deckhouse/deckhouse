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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var staticmachinetemplatelog = logf.Log.WithName("staticmachinetemplate-resource")

func (r *StaticMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

///+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachinetemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachinetemplates,verbs=create;update,versions=v1alpha1,name=mstaticmachinetemplate.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &StaticMachineTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StaticMachineTemplate) Default() {
	staticmachinetemplatelog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachinetemplates,verbs=update,versions=v1alpha1,name=vstaticmachinetemplate.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Validator = &StaticMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachineTemplate) ValidateCreate() (admission.Warnings, error) {
	staticmachinetemplatelog.Info("validate create", "name", r.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachineTemplate) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	staticmachinetemplatelog.Info("validate update", "name", r.Name)

	oldStaticMachineTemplate := old.(*StaticMachineTemplate)
	if !reflect.DeepEqual(r.Spec, oldStaticMachineTemplate.Spec) {
		return nil, field.Forbidden(field.NewPath("spec"), "StaticMachineTemplate.spec is immutable")
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *StaticMachineTemplate) ValidateDelete() (admission.Warnings, error) {
	staticmachinetemplatelog.Info("validate delete", "name", r.Name)

	return nil, nil
}
