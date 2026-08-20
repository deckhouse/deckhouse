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

package nodeconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// nodeRenderInputsChanged reports whether an update touched anything the render
// reads. An event missing either side is passed through: a pass that renders the
// same spec costs a read, and one that never runs costs a node its config.
func nodeRenderInputsChanged(before, after client.Object) bool {
	if before == nil || after == nil {
		return true
	}
	if before.GetUID() != after.GetUID() {
		return true
	}
	createdBefore := before.GetCreationTimestamp()
	createdAfter := after.GetCreationTimestamp()
	if !createdBefore.Equal(&createdAfter) {
		return true
	}
	return !maps.Equal(before.GetLabels(), after.GetLabels())
}

// renderSpec turns a NodeGroup plus cluster state into one node's desired state.
// On the first master it must differ from the installer payload on exactly three
// fields (caCert, serverTLSBootstrap, proxy token); every other field must agree.
func renderSpec(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.NodeSpec {
	return internalv1alpha1.NodeSpec{
		NodeName:           node.Name,
		OSImage:            in.OSImage,
		APIServerEndpoints: in.APIServerEndpoints,
		Extensions:         renderExtensions(in.SysextDigests),
		// A NodeGroup has no disk field; without a selector the boot path refuses
		// outright ("neither device nor diskSelector set"). A richer installer
		// selector survives this value through keepBootstrapOnlyFields.
		Storage:          internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: systemDiskSelectorSize}}},
		Kernel:           renderKernel(),
		Network:          renderNetwork(node),
		Kubelet:          renderKubelet(ng, node, in),
		ContainerRuntime: renderContainerRuntime(ng, in),
		// Every node, not only control-plane: containerd pulls the pause image
		// itself with no imagePullSecret, and in a closed network that image
		// lives in a private registry.
		Registry:                            in.Registry,
		UpdatePolicy:                        renderUpdatePolicy(ng),
		RegistryPackagesProxyAccessTokenB64: in.RegistryPackagesProxyToken,
	}
}

// keepBootstrapOnlyFields carries over the machine-specific parts of the spec —
// kubelet.nodeIP (while still reported), registration taints, explicit network,
// explicit storage. The rest is rendered: a copied value is never corrected.
func keepBootstrapOnlyFields(desired, existing *internalv1alpha1.NodeSpec, reportedNodeIPs []string) {
	if nodeIPStillHolds(existing.Kubelet.NodeIP, reportedNodeIPs) {
		desired.Kubelet.NodeIP = existing.Kubelet.NodeIP
	}

	// Taints take effect while the node registers and are rendered for a machine
	// that has not joined; dropping them afterwards patches every node once,
	// spending a rollout slot on a field no node can act on any more.
	if len(existing.Kubelet.RegisterWithTaints) > 0 {
		desired.Kubelet.RegisterWithTaints = existing.Kubelet.RegisterWithTaints
	}

	// A static network belongs to the provisioner: overwriting it with the
	// rendered "eth0, DHCP" leaves the node unreachable after a reboot.
	// "Explicit" is judged against the render, not the zero value.
	if !sameNetwork(&existing.Network, &desired.Network) {
		hostname := desired.Network.Hostname
		desired.Network = existing.Network
		// The hostname is the cluster's: it must match the Node name however
		// the machine was given its addresses.
		desired.Network.Hostname = hostname
	}

	if storageIsExplicit(&existing.Storage) {
		desired.Storage = existing.Storage
	}
}

// sameNetwork reports whether a node already carries exactly what this render
// would give it. Hostname is left out: it is the cluster's either way.
func sameNetwork(existing, desired *internalv1alpha1.Network) bool {
	a, b := *existing, *desired
	a.Hostname, b.Hostname = "", ""
	return apiequality.Semantic.DeepEqual(a, b)
}

// nodeIPStillHolds reports whether a bootstrapped address is still one the node
// reports: carrying it over unconditionally pinned re-IPed nodes forever. No
// reported addresses means no answer, and the node keeps what it was given.
func nodeIPStillHolds(nodeIP string, reported []string) bool {
	if nodeIP == "" {
		return false
	}
	if len(reported) == 0 {
		return true
	}
	return slices.Contains(reported, nodeIP)
}

// storageIsExplicit reports whether storage carries something this controller
// does not render: only device path and mounts count, deliberately not the disk
// selector — keeping a differing selector would make storage write-once.
func storageIsExplicit(storage *internalv1alpha1.Storage) bool {
	return storage.Device != "" || len(storage.Mounts) > 0
}

// renderKernel publishes the cluster-decided sysctl keys — the same four dhctl
// writes into the first master's payload (dhctl/pkg/immutable/nodeconfig.go).
// Load tuning lives in the OS image (/usr/lib/sysctl.d/50-deckhouse.conf).
func renderKernel() internalv1alpha1.Kernel {
	return internalv1alpha1.Kernel{
		Sysctl: map[string]internalv1alpha1.SysctlValue{
			// kubelet refuses to start without these (protect-kernel-defaults),
			// and fencing changes the delay.
			"kernel.panic":         "10",
			"kernel.panic_on_oops": "1",
			// Pod traffic is routed through the node; mmap count for
			// Elasticsearch and friends. Both exist on every kernel the image
			// can run, so publishing them cannot fail a configuration pass.
			"net.ipv4.ip_forward": "1",
			"vm.max_map_count":    "262144",
		},
	}
}

// renderNetwork keeps the hostname the node booted with. The olcedar init
// renders it from this config on every boot, so losing it here would leave the
// node nameless after a reboot.
func renderNetwork(node *corev1.Node) internalv1alpha1.Network {
	return internalv1alpha1.Network{
		Hostname:   node.Name,
		Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
	}
}

func renderExtensions(digests map[string]string) []internalv1alpha1.Extension {
	names := slices.Sorted(maps.Keys(digests))
	extensions := make([]internalv1alpha1.Extension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, internalv1alpha1.Extension{
			Name:        name,
			Digest:      digests[name],
			RequestedBy: platformExtensionRequestedBy,
		})
	}
	return extensions
}

// renderKubelet maps the NodeGroup's kubelet settings onto the node. Settings
// an olcedar node cannot honour are clamped (a refused config leaves the node
// with none) and reported as Warning events (recordClampedSettings).
func renderKubelet(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.Kubelet {
	kubelet := internalv1alpha1.Kubelet{
		ClusterDomain: in.ClusterDomain,
		NodeLabels:    renderNodeLabels(ng),
		// kubelet reads the CA from disk on every start, and on an immutable
		// node that file is on tmpfs. Without the CA in the config the node
		// cannot rewrite it after a reboot and kubelet never comes back.
		CACert: in.KubernetesCA,
		// Without it the node never gets a providerID and CAPI cannot match its
		// Machine to the Node. Both cloud node types are CAPI-backed, so both
		// need it — not just CloudEphemeral.
		ExternalCloudProvider: isCloudNodeType(ng.Spec.NodeType),
		// Defaults mirror the NodeConfig CRD defaults so the bootstrap file path
		// (no API-server CRD defaulting) gets the same values. maxPods is the
		// exception: it follows the pod subnet the way bashible's does.
		KubernetesVersion:    in.KubernetesVersion,
		MaxPods:              in.DefaultMaxPods,
		ContainerLogMaxSize:  defaultContainerLogMaxSize,
		ContainerLogMaxFiles: defaultContainerLogMaxFiles,
		ResourceReservation:  renderResourceReservation(ng),
	}
	if in.ClusterDNS != "" {
		kubelet.ClusterDNS = []string{in.ClusterDNS}
	}

	if ng.Spec.Kubelet != nil {
		if maxPods, ok := configuredMaxPods(ng); ok {
			kubelet.MaxPods = clampMaxPods(maxPods)
		}
		if positiveQuantity(ng.Spec.Kubelet.ContainerLogMaxSize) {
			kubelet.ContainerLogMaxSize = ng.Spec.Kubelet.ContainerLogMaxSize
		}
		if ng.Spec.Kubelet.ContainerLogMaxFiles != nil {
			kubelet.ContainerLogMaxFiles = int(*ng.Spec.Kubelet.ContainerLogMaxFiles)
		}
	}

	// Taints only take effect while the node registers itself; afterwards the
	// node-template controller owns them, and keepBootstrapOnlyFields carries
	// the rendered value on so a joined node is not patched to drop it.
	if node.CreationTimestamp.IsZero() {
		kubelet.RegisterWithTaints = renderTaints(ng)
	}

	return kubelet
}

func configuredMaxPods(ng *v1.NodeGroup) (int, bool) {
	if ng.Spec.Kubelet == nil || ng.Spec.Kubelet.MaxPods == nil {
		return 0, false
	}
	return int(*ng.Spec.Kubelet.MaxPods), true
}

// clampMaxPods holds the group's number inside what the agent's schema accepts
// (Minimum=1, Maximum=maxPodsCeiling). Outside it the API server rejects the
// whole config and the node is left without one; the clamp is reported.
func clampMaxPods(maxPods int) int {
	return min(max(maxPods, 1), maxPodsCeiling)
}

// positiveQuantity mirrors the CEL rule the agent's schema puts on
// containerLogMaxSize (isQuantity && sign > 0). The NodeGroup pattern is
// unanchored and takes "50Mib" and "0Mi", which that rule refuses outright.
func positiveQuantity(value string) bool {
	quantity, err := resource.ParseQuantity(value)
	return err == nil && quantity.Sign() > 0
}

// renderResourceReservation maps the group's kubeReserved policy onto the node.
// Static is clamped to Auto — the node's schema takes only Auto or Off, and Auto
// is nearer to what was asked for; the clamp is reported (recordClampedSettings).
func renderResourceReservation(ng *v1.NodeGroup) *internalv1alpha1.ResourceReservation {
	mode := resourceReservationModeAuto
	if configured := configuredReservationMode(ng); configured != "" {
		mode = configured
	}
	if mode == resourceReservationModeStatic {
		mode = resourceReservationModeAuto
	}
	return &internalv1alpha1.ResourceReservation{Mode: mode}
}

func configuredReservationMode(ng *v1.NodeGroup) string {
	if ng.Spec.Kubelet == nil || ng.Spec.Kubelet.ResourceReservation == nil {
		return ""
	}
	return ng.Spec.Kubelet.ResourceReservation.Mode
}

// recordClampedSettings tells the operator which NodeGroup settings the render
// did not carry over as written: the agent's schema refuses the value, and
// silent narrowing would leave the group running something nobody configured.
func (r *Reconciler) recordClampedSettings(ng *v1.NodeGroup) {
	if maxPods, ok := configuredMaxPods(ng); ok && clampMaxPods(maxPods) != maxPods {
		r.Recorder.Event(ng, corev1.EventTypeWarning, settingClampedEvent,
			fmt.Sprintf("kubelet.maxPods %d is outside the 1..%d an immutable node accepts; the nodes of this group are configured with %d",
				maxPods, maxPodsCeiling, clampMaxPods(maxPods)))
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode == v1.DisruptionApprovalModeRollingUpdate {
		r.Recorder.Event(ng, corev1.EventTypeWarning, settingClampedEvent,
			fmt.Sprintf("disruptions.approvalMode %s has no counterpart on an immutable node; the nodes of this group are configured with %s",
				v1.DisruptionApprovalModeRollingUpdate, v1.DisruptionApprovalModeAutomatic))
	}
	if configuredReservationMode(ng) == resourceReservationModeStatic {
		r.Recorder.Event(ng, corev1.EventTypeWarning, settingClampedEvent,
			fmt.Sprintf("kubelet.resourceReservation.mode %s cannot be honoured on an immutable node, which reserves by capacity or not at all; the nodes of this group are configured with %s",
				resourceReservationModeStatic, resourceReservationModeAuto))
	}
	if windows := disruptionWindows(ng); len(windows) > 1 {
		r.Recorder.Event(ng, corev1.EventTypeWarning, settingClampedEvent,
			fmt.Sprintf("disruptions windows: an immutable node holds a single update window; the nodes of this group are configured with the first of the %d given",
				len(windows)))
	}
}

// renderNodeLabels returns the labels kubelet registers the node with: the
// group it belongs to, its type, the cgroup layout it runs, and whatever the
// operator asked for.
func renderNodeLabels(ng *v1.NodeGroup) map[string]internalv1alpha1.NodeLabelValue {
	labels := map[string]internalv1alpha1.NodeLabelValue{
		nodecommon.NodeGroupLabel: internalv1alpha1.NodeLabelValue(ng.Name),
		nodecommon.NodeTypeLabel:  internalv1alpha1.NodeLabelValue(ng.Spec.NodeType),
		// An olcedar node is cgroup v2 by construction, and the cluster learns
		// that from this label alone (node-manager raises alerts otherwise).
		// The installer sets it on the first master; the render must keep it.
		cgroupLabel: cgroupV2Value,
	}
	if ng.Spec.NodeTemplate != nil {
		for key, value := range ng.Spec.NodeTemplate.Labels {
			if !kubeletMaySetLabel(key) {
				continue
			}
			labels[key] = internalv1alpha1.NodeLabelValue(value)
		}
	}
	return labels
}

// kubeletMaySetLabel reports whether kubelet accepts the label on --node-labels.
// kubelet exits on a refused label, so one such label leaves the node with no
// kubelet; filtered labels still reach the Node via the node-template controller.
func kubeletMaySetLabel(key string) bool {
	namespace, _, found := strings.Cut(key, "/")
	if !found {
		return true
	}
	// kubelet compares by suffix, so a label under a sub-namespace of a listed
	// one is judged the same way. Matching by equality here dropped legal labels
	// silently — the operator sets one and it is simply not on the node.
	under := func(known string) bool {
		return namespace == known || strings.HasSuffix(namespace, "."+known)
	}
	if !slices.ContainsFunc(reservedLabelNamespaces, under) {
		return true
	}
	if slices.ContainsFunc(kubeletLabelNamespaces, under) {
		return true
	}
	_, allowed := kubeletAllowedLabels[key]
	return allowed
}

func renderTaints(ng *v1.NodeGroup) []internalv1alpha1.Taint {
	if ng.Spec.NodeTemplate == nil || len(ng.Spec.NodeTemplate.Taints) == 0 {
		return nil
	}
	taints := make([]internalv1alpha1.Taint, 0, len(ng.Spec.NodeTemplate.Taints))
	for _, taint := range ng.Spec.NodeTemplate.Taints {
		taints = append(taints, internalv1alpha1.Taint{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: string(taint.Effect),
		})
	}
	return taints
}

// isControlPlaneNode reports whether the node runs the control plane. The label
// is set by the node itself as it brings its apiserver up, and later by the
// node-template controller.
func isControlPlaneNode(node *corev1.Node) bool {
	_, ok := node.Labels[controlPlaneRoleLabel]
	return ok
}

// isCloudNodeType reports whether a node type is CAPI-backed (has a Machine) and
// so needs the external cloud provider to be assigned its providerID.
func isCloudNodeType(t v1.NodeType) bool {
	return t == v1.NodeTypeCloudEphemeral || t == v1.NodeTypeCloudPermanent
}

// renderContainerRuntime carries over the only containerd knob a NodeGroup
// exposes; the runtime itself is a platform-chosen system extension. Defaults
// mirror the CRD defaults so the bootstrap file path gets the same values.
func renderContainerRuntime(ng *v1.NodeGroup, in clusterInputs) internalv1alpha1.ContainerRuntime {
	runtime := internalv1alpha1.ContainerRuntime{
		SandboxImage:           in.SandboxImage,
		MaxConcurrentDownloads: defaultMaxConcurrentDownloads,
	}
	if ng.Spec.CRI == nil {
		return runtime
	}
	switch {
	case ng.Spec.CRI.ContainerdV2 != nil && ng.Spec.CRI.ContainerdV2.MaxConcurrentDownloads != nil:
		runtime.MaxConcurrentDownloads = *ng.Spec.CRI.ContainerdV2.MaxConcurrentDownloads
	case ng.Spec.CRI.Containerd != nil && ng.Spec.CRI.Containerd.MaxConcurrentDownloads != nil:
		runtime.MaxConcurrentDownloads = *ng.Spec.CRI.Containerd.MaxConcurrentDownloads
	}
	return runtime
}

// renderUpdatePolicy maps the group's disruption settings onto the window the
// node may fetch new system extensions in.
func renderUpdatePolicy(ng *v1.NodeGroup) internalv1alpha1.UpdatePolicy {
	policy := internalv1alpha1.UpdatePolicy{Mode: string(v1.DisruptionApprovalModeAutomatic)}
	if ng.Spec.Disruptions == nil {
		return policy
	}
	// RollingUpdate has no counterpart on the node — the agent only knows
	// Manual and Automatic. Passing it through renders a NodeConfig the API
	// server rejects, leaving the node with none at all.
	if mode := ng.Spec.Disruptions.ApprovalMode; mode != "" && mode != v1.DisruptionApprovalModeRollingUpdate {
		policy.Mode = string(mode)
	}

	// NodeConfig carries a single window; the first one the agent's schema takes
	// wins until the agent learns to hold a list. A window it would refuse is
	// skipped: publishing it costs the node its whole config, not one window.
	for _, window := range disruptionWindows(ng) {
		from, fromOK := clockTime(window.From)
		to, toOK := clockTime(window.To)
		if !fromOK || !toOK {
			continue
		}
		policy.Window = internalv1alpha1.UpdateWindow{From: from, To: to, Days: window.Days}
		break
	}
	return policy
}

// clockTime pads a NodeGroup time to the "HH:MM" the agent's schema takes: a
// one-digit hour ("6:00") is legal for a NodeGroup and refused there.
func clockTime(value string) (string, bool) {
	// Go has no layout for a one-digit 24-hour clock: "15" is fixed-width and
	// "3" is the 12-hour one, so the hour is padded before parsing.
	if hour, _, found := strings.Cut(value, ":"); found && len(hour) == 1 {
		value = "0" + value
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return "", false
	}
	return parsed.Format("15:04"), true
}

func disruptionWindows(ng *v1.NodeGroup) []v1.DisruptionWindow {
	if ng.Spec.Disruptions == nil {
		return nil
	}
	if ng.Spec.Disruptions.Automatic != nil {
		return ng.Spec.Disruptions.Automatic.Windows
	}
	if ng.Spec.Disruptions.RollingUpdate != nil {
		return ng.Spec.Disruptions.RollingUpdate.Windows
	}
	return nil
}

// newNodeConfig builds the object for a node, owned by that node so it is
// garbage-collected together with it.
func newNodeConfig(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
			Labels: map[string]string{
				nodecommon.NodeGroupLabel: ng.Name,
				managedByLabel:            managedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
			}},
		},
		Spec: renderSpec(ng, node, in),
	}
}
