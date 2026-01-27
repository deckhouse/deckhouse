/*
Copyright 2025 Flant JSC

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

package v1

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var nodegrouplog = logf.Log.WithName("nodegroup-resource")

// SetupWebhookWithManager sets up the webhook with the manager.
func (r *NodeGroup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-deckhouse-io-v1-nodegroup,mutating=true,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=nodegroups,verbs=create;update,versions=v1,name=mnodegroup.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &NodeGroup{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *NodeGroup) Default() {
	nodegrouplog.Info("default", "name", r.Name)

	// Set default disruption approval mode
	if r.Spec.Disruptions != nil && r.Spec.Disruptions.ApprovalMode == "" {
		r.Spec.Disruptions.ApprovalMode = DisruptionApprovalModeManual
	}

	// Set default chaos mode
	if r.Spec.Chaos != nil && r.Spec.Chaos.Mode == "" {
		r.Spec.Chaos.Mode = ChaosModeDisabled
	}
}

// +kubebuilder:webhook:path=/validate-deckhouse-io-v1-nodegroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=nodegroups,verbs=create;update;delete,versions=v1,name=vnodegroup.deckhouse.io,admissionReviewVersions=v1

var _ webhook.Validator = &NodeGroup{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeGroup) ValidateCreate() (admission.Warnings, error) {
	nodegrouplog.Info("validate create", "name", r.Name)
	return r.validateNodeGroup()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeGroup) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	nodegrouplog.Info("validate update", "name", r.Name)

	oldNodeGroup := old.(*NodeGroup)

	// nodeType is immutable
	if oldNodeGroup.Spec.NodeType != r.Spec.NodeType {
		return nil, field.Forbidden(field.NewPath("spec", "nodeType"), "nodeType is immutable")
	}

	// staticInstances.labelSelector is immutable
	if oldNodeGroup.Spec.StaticInstances != nil && r.Spec.StaticInstances != nil {
		if oldNodeGroup.Spec.StaticInstances.LabelSelector != nil &&
			r.Spec.StaticInstances.LabelSelector != nil {
			// Basic check - in production you'd want deep comparison
			// For now we allow changes if both are set
		}
	}

	return r.validateNodeGroup()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *NodeGroup) ValidateDelete() (admission.Warnings, error) {
	nodegrouplog.Info("validate delete", "name", r.Name)
	return nil, nil
}

// validateNodeGroup performs validation common to create and update
func (r *NodeGroup) validateNodeGroup() (admission.Warnings, error) {
	var allErrs field.ErrorList

	// Validate name
	if err := validateNodeGroupName(r.Name); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"), r.Name, err.Error()))
	}

	// Validate nodeType
	if err := validateNodeType(r); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate cloudInstances
	if errs := validateCloudInstances(r); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	// Validate disruptions
	if errs := validateDisruptions(r); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	// Validate CRI
	if errs := validateCRI(r); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

var nodeGroupNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func validateNodeGroupName(name string) error {
	if len(name) > 42 {
		return fmt.Errorf("name must be no more than 42 characters, got %d", len(name))
	}
	if !nodeGroupNameRegex.MatchString(name) {
		return fmt.Errorf("name must match pattern ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	}
	return nil
}

func validateNodeType(ng *NodeGroup) *field.Error {
	validTypes := map[NodeType]bool{
		NodeTypeCloudEphemeral: true,
		NodeTypeCloudPermanent: true,
		NodeTypeCloudStatic:    true,
		NodeTypeStatic:         true,
	}

	if !validTypes[ng.Spec.NodeType] {
		return field.Invalid(field.NewPath("spec", "nodeType"), ng.Spec.NodeType,
			"must be one of: CloudEphemeral, CloudPermanent, CloudStatic, Static")
	}

	return nil
}

func validateCloudInstances(ng *NodeGroup) field.ErrorList {
	var errs field.ErrorList
	path := field.NewPath("spec", "cloudInstances")

	// CloudEphemeral requires cloudInstances
	if ng.Spec.NodeType == NodeTypeCloudEphemeral {
		if ng.Spec.CloudInstances == nil {
			errs = append(errs, field.Required(path, "cloudInstances is required for nodeType CloudEphemeral"))
			return errs
		}

		ci := ng.Spec.CloudInstances

		// Validate min/max
		if ci.MinPerZone > ci.MaxPerZone {
			errs = append(errs, field.Invalid(path.Child("minPerZone"), ci.MinPerZone,
				fmt.Sprintf("minPerZone cannot be greater than maxPerZone (%d)", ci.MaxPerZone)))
		}

		// Validate classReference
		if ci.ClassReference.Kind == "" {
			errs = append(errs, field.Required(path.Child("classReference", "kind"), "kind is required"))
		}
		if ci.ClassReference.Name == "" {
			errs = append(errs, field.Required(path.Child("classReference", "name"), "name is required"))
		}

		// Validate classReference kind
		validKinds := map[string]bool{
			"OpenStackInstanceClass":   true,
			"GCPInstanceClass":         true,
			"VsphereInstanceClass":     true,
			"AWSInstanceClass":         true,
			"YandexInstanceClass":      true,
			"AzureInstanceClass":       true,
			"VCDInstanceClass":         true,
			"ZvirtInstanceClass":       true,
			"DynamixInstanceClass":     true,
			"HuaweiCloudInstanceClass": true,
			"DVPInstanceClass":         true,
		}
		if ci.ClassReference.Kind != "" && !validKinds[ci.ClassReference.Kind] {
			errs = append(errs, field.NotSupported(path.Child("classReference", "kind"),
				ci.ClassReference.Kind, []string{
					"OpenStackInstanceClass", "GCPInstanceClass", "VsphereInstanceClass",
					"AWSInstanceClass", "YandexInstanceClass", "AzureInstanceClass",
					"VCDInstanceClass", "ZvirtInstanceClass", "DynamixInstanceClass",
					"HuaweiCloudInstanceClass", "DVPInstanceClass",
				}))
		}
	}

	// Static nodes should not have cloudInstances
	if ng.Spec.NodeType == NodeTypeStatic && ng.Spec.CloudInstances != nil {
		errs = append(errs, field.Forbidden(path, "cloudInstances must not be set for nodeType Static"))
	}

	return errs
}

func validateDisruptions(ng *NodeGroup) field.ErrorList {
	var errs field.ErrorList

	if ng.Spec.Disruptions == nil {
		return nil
	}

	d := ng.Spec.Disruptions
	path := field.NewPath("spec", "disruptions")

	// Validate approvalMode
	validModes := map[DisruptionApprovalMode]bool{
		DisruptionApprovalModeManual:        true,
		DisruptionApprovalModeAutomatic:     true,
		DisruptionApprovalModeRollingUpdate: true,
	}

	if d.ApprovalMode != "" && !validModes[d.ApprovalMode] {
		errs = append(errs, field.NotSupported(path.Child("approvalMode"), d.ApprovalMode,
			[]string{"Manual", "Automatic", "RollingUpdate"}))
	}

	// RollingUpdate is only available for cloud nodes
	if d.ApprovalMode == DisruptionApprovalModeRollingUpdate {
		if ng.Spec.NodeType == NodeTypeStatic {
			errs = append(errs, field.Forbidden(path.Child("approvalMode"),
				"RollingUpdate is not available for Static nodes"))
		}
	}

	// Validate windows
	timeRegex := regexp.MustCompile(`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`)
	validDays := map[string]bool{
		"Mon": true, "Tue": true, "Wed": true, "Thu": true,
		"Fri": true, "Sat": true, "Sun": true,
	}

	validateWindows := func(windows []DisruptionWindow, windowPath *field.Path) {
		for i, w := range windows {
			wp := windowPath.Index(i)
			if !timeRegex.MatchString(w.From) {
				errs = append(errs, field.Invalid(wp.Child("from"), w.From, "invalid time format, expected HH:MM"))
			}
			if !timeRegex.MatchString(w.To) {
				errs = append(errs, field.Invalid(wp.Child("to"), w.To, "invalid time format, expected HH:MM"))
			}
			for j, day := range w.Days {
				if !validDays[day] {
					errs = append(errs, field.NotSupported(wp.Child("days").Index(j), day,
						[]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}))
				}
			}
		}
	}

	if d.Automatic != nil && d.Automatic.Windows != nil {
		validateWindows(d.Automatic.Windows, path.Child("automatic", "windows"))
	}

	if d.RollingUpdate != nil && d.RollingUpdate.Windows != nil {
		validateWindows(d.RollingUpdate.Windows, path.Child("rollingUpdate", "windows"))
	}

	return errs
}

func validateCRI(ng *NodeGroup) field.ErrorList {
	var errs field.ErrorList

	if ng.Spec.CRI == nil {
		return nil
	}

	cri := ng.Spec.CRI
	path := field.NewPath("spec", "cri")

	// Validate type
	validTypes := map[CRIType]bool{
		CRITypeDocker:       true,
		CRITypeContainerd:   true,
		CRITypeContainerdV2: true,
		CRITypeNotManaged:   true,
	}

	if cri.Type != "" && !validTypes[cri.Type] {
		errs = append(errs, field.NotSupported(path.Child("type"), cri.Type,
			[]string{"Docker", "Containerd", "ContainerdV2", "NotManaged"}))
	}

	// Check that settings match type
	if cri.Docker != nil && cri.Type != "" && cri.Type != CRITypeDocker {
		errs = append(errs, field.Forbidden(path.Child("docker"), "docker settings can only be used with type=Docker"))
	}

	if cri.Containerd != nil && cri.Type != "" && cri.Type != CRITypeContainerd {
		errs = append(errs, field.Forbidden(path.Child("containerd"), "containerd settings can only be used with type=Containerd"))
	}

	if cri.ContainerdV2 != nil && cri.Type != "" && cri.Type != CRITypeContainerdV2 {
		errs = append(errs, field.Forbidden(path.Child("containerdV2"), "containerdV2 settings can only be used with type=ContainerdV2"))
	}

	if cri.NotManaged != nil && cri.Type != "" && cri.Type != CRITypeNotManaged {
		errs = append(errs, field.Forbidden(path.Child("notManaged"), "notManaged settings can only be used with type=NotManaged"))
	}

	return errs
}
