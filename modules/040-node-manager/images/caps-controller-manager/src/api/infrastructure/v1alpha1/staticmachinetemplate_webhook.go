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
	"context"
	"fmt"
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
		WithDefaulter(&StaticMachineTemplateCustomDefaulter{}).
		WithValidator(&StaticMachineTemplateCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
type StaticMachineTemplateCustomDefaulter struct{}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachinetemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachinetemplates,verbs=create;update,versions=v1alpha1,name=mstaticmachinetemplate.deckhouse.io,admissionReviewVersions=v1
var _ webhook.CustomDefaulter = &StaticMachineTemplateCustomDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StaticMachineTemplateCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	staticMachineTemplate, ok := obj.(*StaticMachineTemplate)
	if !ok {
		return fmt.Errorf("expected a StaticMachineTemplate object but got %T", obj)
	}

	staticmachinetemplatelog.Info("default", "name", staticMachineTemplate.GetName())

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
type StaticMachineTemplateCustomValidator struct{}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-staticmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticmachinetemplates,verbs=update,versions=v1alpha1,name=vstaticmachinetemplate.deckhouse.io,admissionReviewVersions=v1
var _ webhook.CustomValidator = &StaticMachineTemplateCustomValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (*StaticMachineTemplateCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	staticMachineTemplate, ok := obj.(*StaticMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a StaticMachineTemplate object but got %T", obj)
	}

	staticmachinetemplatelog.V(2).Info("validate create", "name", staticMachineTemplate.GetName(), "allowed", true)
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (*StaticMachineTemplateCustomValidator) ValidateUpdate(ctx context.Context, new, old runtime.Object) (admission.Warnings, error) {
	staticMachineTemplate, ok := new.(*StaticMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a StaticMachineTemplate object but got %T", new)
	}

	oldStaticMachineTemplate, ok := old.(*StaticMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a StaticMachineTemplate object but got %T", old)
	}


	// Check if old labelSelector is nil or empty
	oldLabelSelector := oldStaticMachineTemplate.Spec.Template.Spec.LabelSelector
	newLabelSelector := staticMachineTemplate.Spec.Template.Spec.LabelSelector

	isOldLabelSelectorEmpty := false
	if oldLabelSelector == nil {
		isOldLabelSelectorEmpty = true
	} else {
		// Check if both MatchLabels and MatchExpressions are empty
		isOldLabelSelectorEmpty = len(oldLabelSelector.MatchLabels) == 0 && len(oldLabelSelector.MatchExpressions) == 0
	}

	// Check if new labelSelector is nil or empty
	isNewLabelSelectorEmpty := false
	if newLabelSelector == nil {
		isNewLabelSelectorEmpty = true
	} else {
		// Check if both MatchLabels and MatchExpressions are empty
		isNewLabelSelectorEmpty = len(newLabelSelector.MatchLabels) == 0 && len(newLabelSelector.MatchExpressions) == 0
	}

	// Allow changes to labelSelector only if old labelSelector is nil/empty (first set)
	// Create copies to compare without labelSelector
	newSpecCopy := staticMachineTemplate.Spec.DeepCopy()
	oldSpecCopy := oldStaticMachineTemplate.Spec.DeepCopy()

	// Clear labelSelector from both specs for comparison
	newSpecCopy.Template.Spec.LabelSelector = nil
	oldSpecCopy.Template.Spec.LabelSelector = nil

	// Only reject if non-labelSelector fields have changed
	if !reflect.DeepEqual(newSpecCopy, oldSpecCopy) {
		err := field.Forbidden(field.NewPath("spec"), "StaticMachineTemplate.spec is immutable (except labelSelector)")
		staticmachinetemplatelog.Error(err, "validate update rejected", "name", staticMachineTemplate.GetName(), "allowed", false)
		return nil, err
	}

	// If old labelSelector is not empty, check if labelSelector has changed
	if !isOldLabelSelectorEmpty {
		// Disallow removal: old has values but new is nil or empty
		if isNewLabelSelectorEmpty {
			err := field.Forbidden(field.NewPath("spec.template.spec.labelSelector"), "labelSelector can be added but cannot be modified or removed once set")
			staticmachinetemplatelog.Error(err, "validate update rejected", "name", staticMachineTemplate.GetName(), "allowed", false)
			return nil, err
		}
		// Disallow modification: old has values and new has different values
		if !reflect.DeepEqual(newLabelSelector, oldLabelSelector) {
			err := field.Forbidden(field.NewPath("spec.template.spec.labelSelector"), "labelSelector can be added but cannot be modified once set")
			staticmachinetemplatelog.Error(err, "validate update rejected", "name", staticMachineTemplate.GetName(), "allowed", false)
			return nil, err
		}
	}

	staticmachinetemplatelog.V(2).Info("validate update accepted", "name", staticMachineTemplate.GetName(), "allowed", true)

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (*StaticMachineTemplateCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	staticMachineTemplate, ok := obj.(*StaticMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a StaticMachineTemplate object but got %T", obj)
	}

	staticmachinetemplatelog.V(2).Info("validate delete", "name", staticMachineTemplate.GetName(), "allowed", true)

	return nil, nil
}
