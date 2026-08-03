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
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// renderSpec turns the operator's intent (a NodeGroup) plus the cluster's own
// state into the desired state of one node. The node-local agent reconciles
// towards this spec and reports back through the object's status.
func renderSpec(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.NodeSpec {
	extraExtensions, extraModules := nodeExtensions(in.NodeExtensionRequests, node, ng.Name)

	kernel := renderKernel()
	kernel.Modules = mergeModules(kernel.Modules, extraModules)

	spec := internalv1alpha1.NodeSpec{
		NodeName:                            node.Name,
		OSImage:                             defaultOSImage,
		APIServerEndpoints:                  in.APIServerEndpoints,
		Extensions:                          mergeExtensions(renderExtensions(in.SysextDigests), extraExtensions),
		Storage:                             renderStorage(),
		Kernel:                              kernel,
		Network:                             renderNetwork(node),
		Kubelet:                             renderKubelet(ng, node, in),
		ContainerRuntime:                    renderContainerRuntime(ng, in),
		Registry:                            renderRegistry(in),
		UpdatePolicy:                        renderUpdatePolicy(ng),
		RegistryPackagesProxyAccessTokenB64: in.RegistryPackagesProxyToken,
	}
	return spec
}

// renderRegistry gives every node its own way to the cluster's registry.
//
// Not only the control-plane ones: containerd pulls the pause image behind
// every pod sandbox itself, with no pod and therefore no imagePullSecret to
// authenticate with. A node without these credentials cannot start a single
// sandbox once the pause image lives in a private registry, which is what a
// closed-network installation looks like.
func renderRegistry(in clusterInputs) *internalv1alpha1.Registry {
	return in.Registry
}

// renderStorage names no disk, deliberately.
//
// This controller renders a NodeGroup, and a NodeGroup has no disk field: it
// has no input for this decision and therefore no business making one. Which
// disk is the system disk is a fact of the machine, observable only on the
// machine and only at boot, and the boot path answers it there — the disk
// already carrying the BOOT/CONFIG/DATA layout is this node's, and failing
// that, the largest disk that is blank and big enough.
//
// It used to render an empty-but-present selector, on the reasoning that a node
// whose stored config names no disk would drop into the emergency shell. That
// reasoning is spent: a selector constraining nothing matches the first disk
// enumerated, and on a platform that attaches its cloud-init drive as an
// ordinary disk of a megabyte, that disk is the answer — the install then dies
// on "device is too small for the boot/config/data layout".
//
// A node the installer or an operator gave a real selector keeps it:
// keepBootstrapOnlyFields carries over an explicit storage, and only an
// explicit one, so this zero value replaces exactly the values that were
// fabricated here.
func renderStorage() internalv1alpha1.Storage {
	return internalv1alpha1.Storage{}
}

// keepBootstrapOnlyFields carries over the parts of the spec that are decided
// once, by whoever provisioned the node, and that this controller has no input
// for: it renders a NodeGroup, and none of these has a NodeGroup field behind
// it. Overwriting them with the rendered zero value is what a wholesale spec
// patch would otherwise do.
//
// It matters on the first master, which is the only node provisioned from a
// dhctl payload rather than from a rendered NodeConfig, and where each of them
// carries a decision the cluster cannot reproduce:
//
//   - kubelet.nodeIP, kubelet.resourceReservation: set by the payload, not
//     derived from anything in the cluster.
//   - storage: the master picks its system disk by size, because it has two
//     disks and "the first non-CDROM one" is not the answer there.
//
// Two fields the payload also sets are deliberately NOT carried over, and both
// used to be:
//
//   - registry: the render produces it for every node from the cluster's own
//     registry secret, so preserving the bootstrap copy would pin the first
//     master to whatever the installer was told months earlier.
//   - kubelet.serverTLSBootstrap: the payload turns it off because nothing
//     approves serving CSRs before Deckhouse exists. Once Deckhouse is there
//     something does, and keeping it off forever left the master with a
//     self-signed serving certificate carrying no IP — which is kubectl exec
//     and kubectl logs broken against that node, for the life of the cluster.
//
// It is applied to every node rather than only to control-plane ones: on a node
// the controller itself provisioned these are empty on both sides, so the rule
// is the same one either way and needs no test for which kind of node this is.
func keepBootstrapOnlyFields(desired, existing *internalv1alpha1.NodeSpec) {
	desired.Kubelet.NodeIP = existing.Kubelet.NodeIP
	desired.Kubelet.ResourceReservation = existing.Kubelet.ResourceReservation

	// An empty rendered selector claims nothing; anything the node already
	// carries is more specific and was chosen with the disk layout in view.
	if !storageIsExplicit(&existing.Storage) {
		return
	}
	desired.Storage = existing.Storage
}

// storageIsExplicit reports whether a storage section names a disk rather than
// standing for "whatever the first usable one is".
func storageIsExplicit(storage *internalv1alpha1.Storage) bool {
	if storage.Device != "" {
		return true
	}
	return storage.DiskSelector != nil && *storage.DiskSelector != internalv1alpha1.DiskSelector{}
}

// renderKernel repeats the sysctl settings the node was bootstrapped with. This
// config replaces the bootstrap one wholesale, and a key that disappears from
// the desired state is restored to its pre-managed value — dropping
// kernel.panic here would stop kubelet from starting after the next restart.
// renderKernel publishes only what the CLUSTER decides about the kernel.
//
// The tuning a node needs to survive load — inotify limits, the ARP table, pid
// and conntrack ceilings, the receive path — is not here: it belongs to the OS
// image, which is where it now lives (olcedar ships it in
// /usr/lib/sysctl.d/50-deckhouse.conf). That image has a fixed kernel, so it
// knows exactly which knobs exist, and systemd applies them at boot, before
// containerd and kubelet start. Rendering the same values from here meant
// publishing one set to nodes whose kernels may differ, and a single knob
// missing on one of them failed its whole configuration pass.
//
// What remains is policy: how long the kernel waits before rebooting after a
// panic is the cluster's decision (fencing), not the image's. Values published
// here are applied on top of the image's, so this is also the seam an operator
// override travels through.
func renderKernel() internalv1alpha1.Kernel {
	return internalv1alpha1.Kernel{
		Sysctl: map[string]internalv1alpha1.SysctlValue{
			// kubelet refuses to start without these (protect-kernel-defaults),
			// and fencing changes the delay.
			"kernel.panic":         "10",
			"kernel.panic_on_oops": "1",
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
	names := make([]string, 0, len(digests))
	for name := range digests {
		names = append(names, name)
	}
	sort.Strings(names)

	extensions := make([]internalv1alpha1.Extension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, internalv1alpha1.Extension{
			Name:        name,
			Digest:      digests[name],
			RequestedBy: controllerName,
		})
	}
	return extensions
}

// renderKubelet maps the kubelet settings a NodeGroup carries onto the node.
// Settings an olcedar node cannot honour are rejected by the admission webhook
// instead of being silently dropped here.
func renderKubelet(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.Kubelet {
	kubelet := internalv1alpha1.Kubelet{
		ClusterDomain: in.ClusterDomain,
		NodeLabels:    renderNodeLabels(ng),
		// kubelet reads the CA from disk on every start, and on an immutable
		// node that file is on tmpfs. Without the CA in the config the node
		// cannot rewrite it after a reboot and kubelet never comes back.
		CACert: in.KubernetesCA,
		// Without it the node never gets a providerID, and CAPI cannot match
		// the Machine it ordered to the Node that registered. Both cloud node
		// types are CAPI-backed (have a Machine), so both need it — not just
		// CloudEphemeral.
		ExternalCloudProvider: isCloudNodeType(ng.Spec.NodeType),
		// Defaults mirror the NodeConfig CRD defaults so the bootstrap path,
		// which marshals the spec to a file instead of creating it through the
		// API server (where CRD defaulting runs), gets the same values.
		KubernetesVersion:    in.KubernetesVersion,
		MaxPods:              defaultMaxPods,
		ContainerLogMaxSize:  defaultContainerLogMaxSize,
		ContainerLogMaxFiles: defaultContainerLogMaxFiles,
	}
	if in.ClusterDNS != "" {
		kubelet.ClusterDNS = []string{in.ClusterDNS}
	}

	if ng.Spec.Kubelet != nil {
		if ng.Spec.Kubelet.MaxPods != nil {
			// Clamped, not passed through: a NodeGroup has no maximum and the
			// documentation suggests 1000 for a /21 pod subnet, while the agent's
			// schema stops at 500. Over it the API server rejects the whole
			// config, and the node is left without one — a worse answer to "too
			// many pods" than the ceiling itself.
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

// renderNodeLabels returns the labels kubelet registers the node with: the
// group it belongs to, its type, and whatever the operator asked for.
func renderNodeLabels(ng *v1.NodeGroup) map[string]internalv1alpha1.NodeLabelValue {
	labels := map[string]internalv1alpha1.NodeLabelValue{
		nodecommon.NodeGroupLabel: internalv1alpha1.NodeLabelValue(ng.Name),
		nodecommon.NodeTypeLabel:  internalv1alpha1.NodeLabelValue(ng.Spec.NodeType),
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
//
// A node may not label itself into the kubernetes.io or k8s.io namespaces
// beyond a fixed set, and kubelet does not ignore the ones it refuses: it
// exits, so a single such label in a NodeGroup template leaves the node with no
// kubelet at all. The master template carries exactly that —
// node-role.kubernetes.io/control-plane and .../master. Those labels still
// reach the Node, applied by the cluster through the node template controller,
// which is the only side allowed to grant a node its role.
func isReservedNamespace(namespace string) bool {
	return reservedNamespaceMatches(namespace, kubernetesLabelNamespace) || reservedNamespaceMatches(namespace, k8sLabelNamespace)
}

// reservedNamespaceMatches repeats how kubelet compares a label namespace with
// one it knows: the namespace itself, or anything under it.
func reservedNamespaceMatches(namespace, known string) bool {
	return namespace == known || strings.HasSuffix(namespace, "."+known)
}

func kubeletMaySetLabel(key string) bool {
	namespace, _, found := strings.Cut(key, "/")
	if !found {
		return true
	}
	if !isReservedNamespace(namespace) {
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
// exposes; the runtime itself is a system extension chosen by the platform. The
// defaults mirror the NodeConfig CRD defaults so the bootstrap path (which
// marshals the spec to a file rather than creating it through the API server,
// where CRD defaulting runs) produces the same values as a day-2 object.
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
	// RollingUpdate is a NodeGroup mode with no counterpart on the node: an
	// immutable node is replaced rather than updated in place, so the rolling
	// is the cluster's business and the agent only ever sees Manual or
	// Automatic. Passing it through renders a NodeConfig the API server
	// rejects, and a node whose config is rejected never gets one at all.
	if mode := ng.Spec.Disruptions.ApprovalMode; mode != "" && mode != v1.DisruptionApprovalModeRollingUpdate {
		policy.Mode = string(mode)
	}

	var windows []v1.DisruptionWindow
	if ng.Spec.Disruptions.Automatic != nil {
		windows = ng.Spec.Disruptions.Automatic.Windows
	} else if ng.Spec.Disruptions.RollingUpdate != nil {
		windows = ng.Spec.Disruptions.RollingUpdate.Windows
	}
	// NodeConfig carries a single window; the first one wins until the agent
	// learns to hold a list.
	if len(windows) > 0 {
		policy.Window = internalv1alpha1.UpdateWindow{
			From: windows[0].From,
			To:   windows[0].To,
			Days: windows[0].Days,
		}
	}
	return policy
}

// newNodeConfig builds the object for a node, owned by that node so it is
// garbage-collected together with it.
func newNodeConfig(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
			Labels: map[string]string{
				nodeGroupNameLabel: ng.Name,
				managedByLabel:     managedByValue,
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
