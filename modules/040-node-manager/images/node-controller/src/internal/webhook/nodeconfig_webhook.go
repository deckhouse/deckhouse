/*
Copyright 2026 Flant JSC

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

package webhook

import (
	"context"
	"fmt"
	"net/http"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// NodeConfigValidator freezes the machine-owned fields of a NodeConfig.
type NodeConfigValidator struct {
	decoder admission.Decoder
}

// Handle implements admission.Handler. Only UPDATE is registered.
func (w *NodeConfigValidator) Handle(_ context.Context, req admission.Request) admission.Response {
	updated := &internalv1alpha1.NodeConfig{}
	if err := w.decoder.Decode(req, updated); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	old := &internalv1alpha1.NodeConfig{}
	if err := w.decoder.DecodeRaw(req.OldObject, old); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := validateNodeConfigUpdate(old, updated); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

// validateNodeConfigUpdate refuses a change to the fields only the machine
// knows. They arrive when the node registers and never change afterwards: the
// cluster cannot see the hardware, and a wrong value here is a node without a
// route and without a shell.
//
// Two fields the machine publishes are deliberately left out, because the
// controller renders them on every pass and freezing them here would deadlock
// it: the hostname, which must match the Node name, and the disk selector,
// which the cluster renders a guess for when the machine named none.
func validateNodeConfigUpdate(old, updated *internalv1alpha1.NodeConfig) error {
	if err := validateNetworkUpdate(&old.Spec.Network, &updated.Spec.Network); err != nil {
		return err
	}
	if old.Spec.Storage.Device != updated.Spec.Storage.Device ||
		old.Spec.Storage.Wipe != updated.Spec.Storage.Wipe ||
		!apiequality.Semantic.DeepEqual(old.Spec.Storage.Mounts, updated.Spec.Storage.Mounts) {
		return fmt.Errorf("spec.storage names the disk this system was installed on and cannot be " +
			"changed here; delete this NodeConfig and let the node publish what it has")
	}
	return nil
}

// validateNetworkUpdate refuses every network an update writes but the
// machine's own. A machine publishes its addressing when it creates the object,
// so no update ever has one to introduce; the sole exception is the fallback
// the controller renders for a machine that named none, which says nothing only
// the machine could know.
func validateNetworkUpdate(old, updated *internalv1alpha1.Network) error {
	if sameNetworkBesidesHostname(old, updated) {
		return nil
	}
	if !sameNetworkBesidesHostname(old, &internalv1alpha1.Network{}) {
		return fmt.Errorf("spec.network is written on the machine and cannot be changed here; " +
			"delete this NodeConfig and let the node publish what it has")
	}
	fallback := clusterFallbackNetwork()
	if !sameNetworkBesidesHostname(updated, &fallback) {
		return fmt.Errorf("spec.network is written on the machine and cannot be filled in here; " +
			"delete this NodeConfig and let the node publish what it has")
	}
	return nil
}

// clusterFallbackNetwork mirrors renderNetwork in
// internal/controller/nodeconfig/render.go: what the cluster writes into a
// NodeConfig whose machine named no network of its own.
func clusterFallbackNetwork() internalv1alpha1.Network {
	return internalv1alpha1.Network{
		Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
	}
}

// sameNetworkBesidesHostname compares what the machine owns. The hostname is
// the cluster's: renderNetwork sets it to the Node name on every pass.
func sameNetworkBesidesHostname(old, updated *internalv1alpha1.Network) bool {
	a, b := *old, *updated
	a.Hostname, b.Hostname = "", ""
	return apiequality.Semantic.DeepEqual(a, b)
}
