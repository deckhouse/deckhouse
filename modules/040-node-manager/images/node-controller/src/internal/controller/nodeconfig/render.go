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
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// renderSpec turns the operator's intent (a NodeGroup) plus the cluster's own
// state into the desired state of one node. The node-local agent reconciles
// towards this spec and reports back through the object's status.
//
// On the first master it renders over a spec the installer wrote, and that first
// managed render always differs from it, on exactly three fields that the
// payload structurally cannot carry:
//
//   - kubelet.caCert: the cluster CA does not exist when the payload is built.
//     That node generates the cluster PKI itself, which is the point of the
//     design — the installer never holds the key — so the earliest the CA can be
//     known is after the node has booted and brought the control plane up.
//   - kubelet.serverTLSBootstrap: the payload turns it off because nothing
//     approves serving CSRs before Deckhouse exists, and the cluster turns it on
//     once something does. The difference is the state change, not a mismatch.
//   - registryPackagesProxyAccessTokenB64: the secret it comes from is created
//     by Deckhouse, which is not installed yet.
//
// So the aim here is not a no-op — it cannot be. It is that every other field
// agrees, and that each of those three differs for a reason that can be named. A
// field disagreeing for no reason is not a free difference: it is a value the
// node then runs, and "the public registry" or "120 pods" or "no ip_forward" is
// wrong whether or not anyone notices the rollout slot it costs.
//
// Known limitation, not a plan: between bootstrap and that first managed render
// the master has no CA in its NodeConfig. For a node that receives its CA this
// way that matters — the file lives on tmpfs, so it is rewritten from the config
// on every boot (see clusterInputs.KubernetesCA). Whether it matters for the
// master, which generated the PKI itself and keeps it on its own control-plane
// storage, depends on where nodelet keeps what it generated, which is not
// answerable from this repository.
func renderSpec(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.NodeSpec {
	extraExtensions, extraModules := nodeExtensions(in.NodeExtensionRequests, node, ng.Name)

	kernel := renderKernel()
	kernel.Modules = mergeModules(kernel.Modules, extraModules)

	spec := internalv1alpha1.NodeSpec{
		NodeName:           node.Name,
		OSImage:            in.OSImage,
		APIServerEndpoints: in.APIServerEndpoints,
		Extensions:         mergeExtensions(renderExtensions(in.SysextDigests), extraExtensions),
		// A NodeGroup has no disk field, so the only true statement this
		// controller can make about the machine is systemDiskSelector: any real
		// disk, none of the attach junk. Without a selector the boot path
		// refuses outright ("neither device nor diskSelector set" — measured, a
		// worker stuck in the initramfs shell), and the machines this reaches
		// have exactly one real disk, so the selector is unambiguous. A selector
		// the installer chose is richer (it separates the master's system disk
		// from the etcd disk) and survives this value through
		// keepBootstrapOnlyFields.
		Storage:          internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: systemDiskSelectorSize}}},
		Kernel:           kernel,
		Network:          renderNetwork(node),
		Kubelet:          renderKubelet(ng, node, in),
		ContainerRuntime: renderContainerRuntime(ng, in),
		// Every node, not only the control-plane ones: containerd pulls the pause
		// image behind every pod sandbox itself, with no pod and so no
		// imagePullSecret, and in a closed network that image is in a private
		// registry.
		Registry:                            in.Registry,
		UpdatePolicy:                        renderUpdatePolicy(ng),
		RegistryPackagesProxyAccessTokenB64: in.RegistryPackagesProxyToken,
	}
	return spec
}

// keepBootstrapOnlyFields carries over the parts of the spec that describe the
// machine rather than the group, which is what this controller renders. They
// belong to whoever provisioned the node; overwriting them with the rendered
// value is what a wholesale spec patch would otherwise do.
//
//   - kubelet.nodeIP: the address the node registers under, kept only while the
//     node still reports it (nodeIPStillHolds).
//   - network: static addressing, DNS, NTP and routes, kept when they name
//     anything beyond the rendered eth0-on-DHCP (networkIsExplicit). The
//     hostname stays the cluster's either way.
//   - storage: which disk to install onto, kept only when it names something
//     richer than the rendered selector (storageIsExplicit) — the installer's
//     master payload does: its selector separates the system disk from the etcd
//     disk, and its mounts are where /var/lib/etcd lives.
//
// Beyond the installer, the same NodeConfig can be pushed to a node by hand
// through nodelet's maintenance endpoint, which is where a machine-specific
// address or network would come from; a render that dropped them would undo
// that push on its next pass.
//
// Everything else the payload sets is rendered rather than preserved, including
// two that used to be carried over and must not be: registry (preserving it
// pinned the first master to whatever the installer was told months earlier) and
// kubelet.serverTLSBootstrap (the payload turns it off because nothing approves
// serving CSRs before Deckhouse exists; keeping it off forever left the master
// with a self-signed serving certificate carrying no IP, so no kubectl exec and
// no kubectl logs against that node for the life of the cluster). A value that is
// only ever copied from the object's own previous state can never be corrected,
// because after the first patch nothing can tell an installer's value from one an
// earlier pass carried over.
//
// It is applied to every node rather than only to control-plane ones: on a node
// the controller itself provisioned these are empty on both sides, so the rule
// is the same one either way and needs no test for which kind of node this is.
func keepBootstrapOnlyFields(desired, existing *internalv1alpha1.NodeSpec, reportedNodeIPs []string) {
	if nodeIPStillHolds(existing.Kubelet.NodeIP, reportedNodeIPs) {
		desired.Kubelet.NodeIP = existing.Kubelet.NodeIP
	}

	// A statically configured network belongs to whoever provisioned the
	// machine: the render only ever says "eth0, DHCP", and overwriting a
	// static address with that leaves the node unreachable after its next
	// reboot — the OS renders its network from this spec.
	//
	// "Somebody else chose it" is decided against what this controller would
	// render, not against the zero value: a rendered value that also counts as
	// explicit is one this controller can never correct again, because after
	// the first patch nothing tells it apart from a provisioner's.
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

// nodeIPStillHolds reports whether an address the node was bootstrapped with is
// still one of its own. Nothing else ever writes kubelet.nodeIP, so carrying it
// over unconditionally pinned a node to the address it first had — a node that
// is re-IPed (DHCP lease, migration, a rebuilt VM) would register forever under
// an address that is no longer routed to it. A node that reports no address at
// all says nothing either way, and keeps what it was given.
//
// The addresses are the ones the node itself reports, read from the API server
// (see reportedNodeIPs): the manager's cache strips Node.status.addresses, so
// judging by the cached Node means judging by "this node has no addresses" for
// every node in the cluster — which keeps every stale pin forever, the exact
// defect this rule exists to undo.
func nodeIPStillHolds(nodeIP string, reported []string) bool {
	if nodeIP == "" {
		return false
	}
	if len(reported) == 0 {
		return true
	}
	return slices.Contains(reported, nodeIP)
}

// storageIsExplicit reports whether a node's storage carries something this
// controller does not render, and so belongs to whoever provisioned the machine.
//
// Only a device path and mounts count — deliberately not the disk selector,
// which this controller renders itself. Keeping a selector because it differs
// from the rendered one makes storage write-once: the threshold could never be
// corrected on a node that already exists, and this file warns against exactly
// that a few lines up — a value only ever copied from its own previous state
// can never be fixed.
//
// Nothing is lost by re-rendering it. The installer's master payload carries
// mounts as well, so the whole section — its richer selector included — is kept
// by the first test; and on a worker nobody but this controller writes one.
//
// Mounts are the case that matters in practice: a control-plane node is given
// the disk etcd lives on through them, and a render that dropped the list would
// leave the node with nothing under /var/lib/etcd on its next pass — etcd would
// come up as a brand new cluster after the next reboot.
func storageIsExplicit(storage *internalv1alpha1.Storage) bool {
	return storage.Device != "" || len(storage.Mounts) > 0
}

// renderKernel publishes what the CLUSTER decides about the kernel. This config
// replaces the bootstrap one wholesale, and a key that disappears from the
// desired state is restored to its pre-managed value — so every key a node must
// keep running has to be published here. That is the whole list, and it is the
// same four the installer writes into the first master's payload
// (dhctl/pkg/immutable/nodeconfig.go): publishing fewer would take a working
// setting away from that master, and every other immutable node would run a
// kernel configured differently from the one the cluster was brought up on.
//
// Publishing them, rather than carrying over whatever the node happens to have,
// is what keeps them changeable: a value that is only ever copied from the
// object's previous state can never be corrected, because after the first patch
// nothing can tell an installer's value from one an earlier pass carried over.
//
// The tuning a node needs to survive load — inotify limits, the ARP table, pid
// and conntrack ceilings, the receive path — is not here: it belongs to the OS
// image, which is where it now lives (olcedar ships it in
// /usr/lib/sysctl.d/50-deckhouse.conf). That image has a fixed kernel, so it
// knows exactly which knobs exist, and systemd applies them at boot, before
// containerd and kubelet start. Rendering the same values from here meant
// publishing one set to nodes whose kernels may differ, and a single knob
// missing on one of them failed its whole configuration pass.
func renderKernel() internalv1alpha1.Kernel {
	return internalv1alpha1.Kernel{
		Sysctl: map[string]internalv1alpha1.SysctlValue{
			// kubelet refuses to start without these (protect-kernel-defaults),
			// and fencing changes the delay.
			"kernel.panic":         "10",
			"kernel.panic_on_oops": "1",
			// Pod traffic is routed through the node, and a pod's mmap count is
			// not the host default (Elasticsearch and friends need this one).
			// Both exist on every kernel the image can run, so publishing them
			// cannot fail a node's configuration pass.
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
			RequestedBy: platformExtensionRequestedBy,
		})
	}
	return extensions
}

// renderKubelet maps the kubelet settings a NodeGroup carries onto the node.
// A setting an olcedar node cannot honour as written is clamped to what its
// schema accepts rather than passed through — a config the API server refuses
// leaves the node with no config at all. What was clamped is reported on the
// NodeGroup as a Warning event (see recordClampedSettings), because a setting
// quietly narrowed is a group running something nobody configured.
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
		// API server (where CRD defaulting runs), gets the same values. maxPods
		// is the exception: it follows the pod subnet, the way bashible's does
		// (see defaultMaxPodsFor), so nodes of both kinds advertise one capacity.
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

// registrationLabels is what renderNodeLabels publishes, as a Node carries it:
// the labels kubelet will register the node with, which is what a
// NodeExtensionRequest selecting by node label has to match against before the
// node exists.
func registrationLabels(ng *v1.NodeGroup) map[string]string {
	rendered := renderNodeLabels(ng)
	labels := make(map[string]string, len(rendered))
	for key, value := range rendered {
		labels[key] = string(value)
	}
	return labels
}

// renderResourceReservation maps the group's kubeReserved policy onto the node.
//
// It is rendered rather than carried over from whatever the node already has:
// the NodeGroup has this field, so an operator turning the reservation off has
// to reach the nodes, and a value only ever copied from the object's previous
// state cannot be changed by anyone once it is there.
//
// Static is clamped to Auto: a NodeGroup can name exact amounts, an immutable
// node's schema takes only Auto or Off, and Auto reserves something rather than
// nothing, which is the nearer of the two to what was asked for. The clamp is
// reported on the NodeGroup (see recordClampedSettings).
func renderResourceReservation(ng *v1.NodeGroup) *internalv1alpha1.ResourceReservation {
	mode := resourceReservationModeAuto
	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.ResourceReservation != nil && ng.Spec.Kubelet.ResourceReservation.Mode != "" {
		mode = ng.Spec.Kubelet.ResourceReservation.Mode
	}
	if mode == resourceReservationModeStatic {
		mode = resourceReservationModeAuto
	}
	return &internalv1alpha1.ResourceReservation{Mode: mode}
}

// renderNodeLabels returns the labels kubelet registers the node with: the
// group it belongs to, its type, the cgroup layout it runs, and whatever the
// operator asked for.
func renderNodeLabels(ng *v1.NodeGroup) map[string]internalv1alpha1.NodeLabelValue {
	labels := map[string]internalv1alpha1.NodeLabelValue{
		nodecommon.NodeGroupLabel: internalv1alpha1.NodeLabelValue(ng.Name),
		nodecommon.NodeTypeLabel:  internalv1alpha1.NodeLabelValue(ng.Spec.NodeType),
		// An olcedar node is cgroup v2 by construction, and the cluster learns
		// that from this label alone: node-manager reads it off the Node and
		// counts anything else as a node that cannot run containerd v2, raising
		// d8_node_cgroup_v2_unsupported and nodeManager:unsupportedContainerdV1.
		// The installer sets it on the first master, so leaving it out of the
		// render both took it away from that node on its first managed render and
		// meant no rendered node ever had it.
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

// isReservedNamespace reports whether a label namespace is one kubelet guards.
func isReservedNamespace(namespace string) bool {
	return reservedNamespaceMatches(namespace, kubernetesLabelNamespace) || reservedNamespaceMatches(namespace, k8sLabelNamespace)
}

// reservedNamespaceMatches repeats how kubelet compares a label namespace with
// one it knows: the namespace itself, or anything under it.
func reservedNamespaceMatches(namespace, known string) bool {
	return namespace == known || strings.HasSuffix(namespace, "."+known)
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
