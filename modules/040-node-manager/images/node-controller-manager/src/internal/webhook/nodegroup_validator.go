package webhook

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	admissionv1 "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	nodev1 "github.com/deckhouse/node-controller/api/v1"
)

const (
	validatePath = "/validate-deckhouse-io-v1-nodegroup"
)

// +kubebuilder:webhook:path=/validate-deckhouse-io-v1-nodegroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=nodegroups,verbs=create;update;delete,versions=v1,name=vnodegroup.deckhouse.io,admissionReviewVersions=v1

// NodeGroupValidator validates NodeGroup resources
type NodeGroupValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupNodeGroupValidator registers the validation webhook
func SetupNodeGroupValidator(mgr ctrl.Manager) error {
	validator := &NodeGroupValidator{
		Client: mgr.GetClient(),
	}

	mgr.GetWebhookServer().Register(validatePath, &webhook.Admission{
		Handler: validator,
	})

	return nil
}

// Handle handles admission requests
func (v *NodeGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Validating NodeGroup", "operation", req.Operation, "name", req.Name)

	nodeGroup := &nodev1.NodeGroup{}

	// For DELETE operation, object is in OldObject
	if req.Operation == admissionv1.Delete {
		if err := v.decoder.DecodeRaw(req.OldObject, nodeGroup); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return v.validateDelete(ctx, nodeGroup)
	}

	if err := v.decoder.Decode(req, nodeGroup); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Create:
		return v.validateCreate(ctx, nodeGroup)
	case admissionv1.Update:
		oldNodeGroup := &nodev1.NodeGroup{}
		if err := v.decoder.DecodeRaw(req.OldObject, oldNodeGroup); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return v.validateUpdate(ctx, oldNodeGroup, nodeGroup)
	default:
		return admission.Allowed("")
	}
}

// InjectDecoder injects the decoder
func (v *NodeGroupValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// validateCreate validates NodeGroup on create
func (v *NodeGroupValidator) validateCreate(ctx context.Context, ng *nodev1.NodeGroup) admission.Response {
	// ==========================================
	// TODO: MIGRATE VALIDATION FROM PYTHON/BASH WEBHOOKS
	// ==========================================

	// Validate name format
	if err := validateNodeGroupName(ng.Name); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate nodeType
	if err := validateNodeType(ng); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate cloudInstances for CloudEphemeral
	if err := validateCloudInstances(ng); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate disruptions
	if err := validateDisruptions(ng); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate CRI settings
	if err := validateCRI(ng); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// validateUpdate validates NodeGroup on update
func (v *NodeGroupValidator) validateUpdate(ctx context.Context, oldNG, newNG *nodev1.NodeGroup) admission.Response {
	// ==========================================
	// TODO: MIGRATE UPDATE VALIDATION FROM PYTHON/BASH
	// ==========================================

	// Check immutable fields
	if oldNG.Spec.NodeType != newNG.Spec.NodeType {
		return admission.Denied("spec.nodeType is immutable and cannot be changed")
	}

	// staticInstances.labelSelector is immutable
	if oldNG.Spec.StaticInstances != nil && newNG.Spec.StaticInstances != nil {
		// LabelSelector immutability is handled by CEL validation in CRD
		// but we can add extra validation here if needed
	}

	// Run create validations
	return v.validateCreate(ctx, newNG)
}

// validateDelete validates NodeGroup on delete
func (v *NodeGroupValidator) validateDelete(ctx context.Context, ng *nodev1.NodeGroup) admission.Response {
	// ==========================================
	// TODO: MIGRATE DELETE VALIDATION FROM PYTHON/BASH
	// ==========================================

	// Example: Check if there are nodes in the group before deletion
	// This could be added if you want to prevent deletion of non-empty groups

	return admission.Allowed("")
}

// =====================
// Validation functions
// =====================

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

func validateNodeType(ng *nodev1.NodeGroup) error {
	validTypes := map[nodev1.NodeType]bool{
		nodev1.NodeTypeCloudEphemeral: true,
		nodev1.NodeTypeCloudPermanent: true,
		nodev1.NodeTypeCloudStatic:    true,
		nodev1.NodeTypeStatic:         true,
	}

	if !validTypes[ng.Spec.NodeType] {
		return fmt.Errorf("invalid nodeType: %s, must be one of: CloudEphemeral, CloudPermanent, CloudStatic, Static", ng.Spec.NodeType)
	}

	return nil
}

func validateCloudInstances(ng *nodev1.NodeGroup) error {
	// CloudInstances is required for CloudEphemeral
	if ng.Spec.NodeType == nodev1.NodeTypeCloudEphemeral {
		if ng.Spec.CloudInstances == nil {
			return fmt.Errorf("spec.cloudInstances is required for nodeType CloudEphemeral")
		}

		ci := ng.Spec.CloudInstances

		// Validate min/max
		if ci.MinPerZone > ci.MaxPerZone {
			return fmt.Errorf("spec.cloudInstances.minPerZone (%d) cannot be greater than maxPerZone (%d)",
				ci.MinPerZone, ci.MaxPerZone)
		}

		// Validate classReference
		if ci.ClassReference.Kind == "" {
			return fmt.Errorf("spec.cloudInstances.classReference.kind is required")
		}
		if ci.ClassReference.Name == "" {
			return fmt.Errorf("spec.cloudInstances.classReference.name is required")
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
		if !validKinds[ci.ClassReference.Kind] {
			return fmt.Errorf("invalid classReference.kind: %s", ci.ClassReference.Kind)
		}
	}

	// CloudInstances should not be set for Static nodes
	if ng.Spec.NodeType == nodev1.NodeTypeStatic && ng.Spec.CloudInstances != nil {
		return fmt.Errorf("spec.cloudInstances must not be set for nodeType Static")
	}

	return nil
}

func validateDisruptions(ng *nodev1.NodeGroup) error {
	if ng.Spec.Disruptions == nil {
		return nil
	}

	d := ng.Spec.Disruptions

	// Validate approvalMode
	validModes := map[nodev1.DisruptionApprovalMode]bool{
		nodev1.DisruptionApprovalModeManual:        true,
		nodev1.DisruptionApprovalModeAutomatic:     true,
		nodev1.DisruptionApprovalModeRollingUpdate: true,
	}

	if d.ApprovalMode != "" && !validModes[d.ApprovalMode] {
		return fmt.Errorf("invalid disruptions.approvalMode: %s", d.ApprovalMode)
	}

	// RollingUpdate is only available for cloud nodes
	if d.ApprovalMode == nodev1.DisruptionApprovalModeRollingUpdate {
		if ng.Spec.NodeType == nodev1.NodeTypeStatic {
			return fmt.Errorf("disruptions.approvalMode RollingUpdate is not available for Static nodes")
		}
	}

	// Validate windows time format
	if d.Automatic != nil {
		for _, w := range d.Automatic.Windows {
			if err := validateTimeWindow(w); err != nil {
				return fmt.Errorf("invalid automatic window: %v", err)
			}
		}
	}

	if d.RollingUpdate != nil {
		for _, w := range d.RollingUpdate.Windows {
			if err := validateTimeWindow(w); err != nil {
				return fmt.Errorf("invalid rollingUpdate window: %v", err)
			}
		}
	}

	return nil
}

var timeRegex = regexp.MustCompile(`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`)
var validDays = map[string]bool{
	"Mon": true, "Tue": true, "Wed": true, "Thu": true,
	"Fri": true, "Sat": true, "Sun": true,
}

func validateTimeWindow(w nodev1.DisruptionWindow) error {
	if !timeRegex.MatchString(w.From) {
		return fmt.Errorf("invalid 'from' time format: %s, expected HH:MM", w.From)
	}
	if !timeRegex.MatchString(w.To) {
		return fmt.Errorf("invalid 'to' time format: %s, expected HH:MM", w.To)
	}
	for _, day := range w.Days {
		if !validDays[day] {
			return fmt.Errorf("invalid day: %s", day)
		}
	}
	return nil
}

func validateCRI(ng *nodev1.NodeGroup) error {
	if ng.Spec.CRI == nil {
		return nil
	}

	cri := ng.Spec.CRI

	// Validate type
	validTypes := map[nodev1.CRIType]bool{
		nodev1.CRITypeDocker:       true,
		nodev1.CRITypeContainerd:   true,
		nodev1.CRITypeContainerdV2: true,
		nodev1.CRITypeNotManaged:   true,
	}

	if cri.Type != "" && !validTypes[cri.Type] {
		return fmt.Errorf("invalid cri.type: %s", cri.Type)
	}

	// Docker-specific settings require Docker type
	if cri.Docker != nil && cri.Type != "" && cri.Type != nodev1.CRITypeDocker {
		return fmt.Errorf("cri.docker can only be used with cri.type=Docker")
	}

	// Containerd-specific settings require Containerd type
	if cri.Containerd != nil && cri.Type != "" && cri.Type != nodev1.CRITypeContainerd {
		return fmt.Errorf("cri.containerd can only be used with cri.type=Containerd")
	}

	// ContainerdV2-specific settings require ContainerdV2 type
	if cri.ContainerdV2 != nil && cri.Type != "" && cri.Type != nodev1.CRITypeContainerdV2 {
		return fmt.Errorf("cri.containerdV2 can only be used with cri.type=ContainerdV2")
	}

	// NotManaged-specific settings require NotManaged type
	if cri.NotManaged != nil && cri.Type != "" && cri.Type != nodev1.CRITypeNotManaged {
		return fmt.Errorf("cri.notManaged can only be used with cri.type=NotManaged")
	}

	return nil
}
