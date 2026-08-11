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
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// renderSpec turns a NodeGroup plus cluster state into one node's desired state.
// On the first master it must differ from the installer payload on exactly three
// fields (caCert, serverTLSBootstrap, proxy token); every other field must agree.
func renderSpec(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.NodeSpec {
	extraExtensions, extraModules := nodeExtensions(in.NodeExtensions, in.NodeExtensionConflicts, node, ng.Name)

	kernel := renderKernel()
	kernel.Modules = extraModules

	return internalv1alpha1.NodeSpec{
		NodeName:           node.Name,
		OSImage:            in.OSImage,
		APIServerEndpoints: in.APIServerEndpoints,
		Extensions:         mergeExtensions(renderExtensions(in.SysextDigests), extraExtensions),
		// A NodeGroup has no disk field; without a selector the boot path refuses
		// outright ("neither device nor diskSelector set"). A richer installer
		// selector survives this value through keepBootstrapOnlyFields.
		Storage:          internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: systemDiskSelectorSize}}},
		Kernel:           kernel,
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
// kubelet.nodeIP (while still reported), explicit network, explicit storage.
// Everything else is rendered: a value only ever copied can never be corrected.
func keepBootstrapOnlyFields(desired, existing *internalv1alpha1.NodeSpec, reportedNodeIPs []string) {
	if nodeIPStillHolds(existing.Kubelet.NodeIP, reportedNodeIPs) {
		desired.Kubelet.NodeIP = existing.Kubelet.NodeIP
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

// renderExtensions lists the system extensions the node merges onto its
// read-only root, pinned by digest.
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
		if ng.Spec.Kubelet.MaxPods != nil {
			// Clamped, not passed through: a NodeGroup has no maximum while the
			// agent's schema stops at maxPodsCeiling; over it the API server
			// rejects the whole config and the node is left without one.
			kubelet.MaxPods = min(int(*ng.Spec.Kubelet.MaxPods), maxPodsCeiling)
		}
		if ng.Spec.Kubelet.ContainerLogMaxSize != "" {
			kubelet.ContainerLogMaxSize = ng.Spec.Kubelet.ContainerLogMaxSize
		}
		if ng.Spec.Kubelet.ContainerLogMaxFiles != nil {
			kubelet.ContainerLogMaxFiles = int(*ng.Spec.Kubelet.ContainerLogMaxFiles)
		}
	}

	// Taints only take effect while the node registers itself, so they are
	// rendered for a node that has not joined yet. Afterwards the node-template
	// controller owns the taints on the Node object.
	if node.CreationTimestamp.IsZero() {
		kubelet.RegisterWithTaints = renderTaints(ng)
	}

	return kubelet
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

// configuredReservationMode returns the kubeReserved mode the group asks for,
// empty when it asks for none.
func configuredReservationMode(ng *v1.NodeGroup) string {
	if ng.Spec.Kubelet == nil || ng.Spec.Kubelet.ResourceReservation == nil {
		return ""
	}
	return ng.Spec.Kubelet.ResourceReservation.Mode
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

// reservedNamespaceMatches repeats how kubelet compares a label namespace with
// one it knows: the namespace itself, or anything under it.
func reservedNamespaceMatches(namespace, known string) bool {
	return namespace == known || strings.HasSuffix(namespace, "."+known)
}

// kubeletMaySetLabel reports whether kubelet accepts the label on --node-labels.
// kubelet exits on a refused label, so one such label leaves the node with no
// kubelet; filtered labels still reach the Node via the node-template controller.
func kubeletMaySetLabel(key string) bool {
	namespace, _, found := strings.Cut(key, "/")
	if !found {
		return true
	}
	if !reservedNamespaceMatches(namespace, kubernetesLabelNamespace) && !reservedNamespaceMatches(namespace, k8sLabelNamespace) {
		return true
	}
	// kubelet compares by suffix, so a label under a sub-namespace of an allowed
	// prefix is allowed too. Matching by equality here dropped legal labels
	// silently — the operator sets one and it is simply not on the node.
	if reservedNamespaceMatches(namespace, kubeletLabelNamespace) || reservedNamespaceMatches(namespace, nodeLabelNamespace) {
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

	// NodeConfig carries a single window; the first one wins until the agent
	// learns to hold a list.
	if windows := disruptionWindows(ng); len(windows) > 0 {
		policy.Window = internalv1alpha1.UpdateWindow{
			From: windows[0].From,
			To:   windows[0].To,
			Days: windows[0].Days,
		}
	}
	return policy
}

// disruptionWindows returns the update windows the group configured: the
// automatic ones, or the rolling-update ones when it configures no automatic.
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
