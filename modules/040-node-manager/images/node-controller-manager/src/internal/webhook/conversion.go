package webhook

import (
	ctrl "sigs.k8s.io/controller-runtime"

	nodev1 "github.com/deckhouse/node-controller/api/v1"
)

// SetupConversionWebhook registers the conversion webhook
// Note: controller-runtime handles conversion automatically when types implement
// conversion.Convertible interface (ConvertTo/ConvertFrom methods)
func SetupConversionWebhook(mgr ctrl.Manager) error {
	// The conversion webhook is automatically set up by controller-runtime
	// when the CRD has conversion webhook configuration and the types
	// implement the conversion.Convertible interface.
	//
	// The conversion methods are defined in:
	// - api/v1alpha1/nodegroup_conversion.go
	// - api/v1alpha2/nodegroup_conversion.go
	//
	// v1 is the "hub" version (storage version), and v1alpha1/v1alpha2
	// convert to/from v1.

	// Register v1 as the hub for conversion
	// This is done implicitly - v1 is the storage version in the CRD

	// For explicit conversion webhook registration (if needed):
	// mgr.GetWebhookServer().Register("/convert", &webhook.Admission{
	//     Handler: &conversionHandler{},
	// })

	// The actual conversion happens via the Convertible interface:
	// - v1alpha1.NodeGroup implements ConvertTo(*v1.NodeGroup) and ConvertFrom(*v1.NodeGroup)
	// - v1alpha2.NodeGroup implements ConvertTo(*v1.NodeGroup) and ConvertFrom(*v1.NodeGroup)

	_ = nodev1.NodeGroup{} // Ensure v1 types are imported

	return nil
}

// Conversion logic is implemented in:
// api/v1alpha1/nodegroup_conversion.go - ConvertTo/ConvertFrom between v1alpha1 <-> v1
// api/v1alpha2/nodegroup_conversion.go - ConvertTo/ConvertFrom between v1alpha2 <-> v1
//
// Key conversions:
//
// nodeType mapping:
//   v1alpha1/v1alpha2     ->    v1
//   ─────────────────────────────────
//   Cloud                 ->    CloudEphemeral
//   Static                ->    Static
//   Hybrid                ->    CloudStatic
//
//   v1                    ->    v1alpha1/v1alpha2
//   ─────────────────────────────────
//   CloudEphemeral        ->    Cloud
//   CloudPermanent        ->    Hybrid (lossy!)
//   CloudStatic           ->    Hybrid
//   Static                ->    Static
//
// CRI type mapping:
//   v1                    ->    v1alpha1/v1alpha2
//   ─────────────────────────────────
//   ContainerdV2          ->    Containerd (downgrade)
//
// Fields that exist only in v1 (lost on conversion to old versions):
// - spec.gpu
// - spec.staticInstances
// - spec.fencing
// - spec.update
// - spec.nodeDrainTimeoutSecond
// - spec.kubelet.resourceReservation
// - spec.kubelet.topologyManager
// - spec.kubelet.memorySwap
// - status.conditions
// - status.deckhouse
