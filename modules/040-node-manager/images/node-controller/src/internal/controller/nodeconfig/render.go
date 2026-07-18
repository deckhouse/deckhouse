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
	spec := internalv1alpha1.NodeSpec{
		NodeName:                            node.Name,
		OSImage:                             defaultOSImage,
		APIServerEndpoints:                  in.APIServerEndpoints,
		Extensions:                          renderExtensions(in.SysextDigests),
		Kernel:                              renderKernel(),
		Network:                             renderNetwork(node),
		Kubelet:                             renderKubelet(ng, node, in),
		ContainerRuntime:                    renderContainerRuntime(ng),
		UpdatePolicy:                        renderUpdatePolicy(ng),
		RegistryPackagesProxyAccessTokenB64: in.RegistryPackagesProxyToken,
	}
	return spec
}

// renderKernel repeats the sysctl settings the node was bootstrapped with. This
// config replaces the bootstrap one wholesale, and a key that disappears from
// the desired state is restored to its pre-managed value — dropping
// kernel.panic here would stop kubelet from starting after the next restart.
func renderKernel() internalv1alpha1.Kernel {
	return internalv1alpha1.Kernel{
		Sysctl: map[string]internalv1alpha1.SysctlValue{
			"net.ipv4.ip_forward": "1",
			"vm.max_map_count":    "262144",
			// kubelet refuses to start without these (protect-kernel-defaults).
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
// Settings the immutable OS cannot honour are rejected by the admission webhook
// instead of being silently dropped here.
func renderKubelet(ng *v1.NodeGroup, node *corev1.Node, in clusterInputs) internalv1alpha1.Kubelet {
	kubelet := internalv1alpha1.Kubelet{
		ClusterDomain: in.ClusterDomain,
		NodeLabels:    renderNodeLabels(ng),
		// Without it the node never gets a providerID, and CAPI cannot match
		// the Machine it ordered to the Node that registered.
		ExternalCloudProvider: ng.Spec.NodeType == v1.NodeTypeCloudEphemeral,
	}
	if in.ClusterDNS != "" {
		kubelet.ClusterDNS = []string{in.ClusterDNS}
	}

	if ng.Spec.Kubelet != nil {
		if ng.Spec.Kubelet.MaxPods != nil {
			kubelet.MaxPods = int(*ng.Spec.Kubelet.MaxPods)
		}
		kubelet.ContainerLogMaxSize = ng.Spec.Kubelet.ContainerLogMaxSize
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
			labels[key] = internalv1alpha1.NodeLabelValue(value)
		}
	}
	return labels
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

// renderContainerRuntime carries over the only containerd knob a NodeGroup
// exposes; the runtime itself is a system extension chosen by the platform.
func renderContainerRuntime(ng *v1.NodeGroup) internalv1alpha1.ContainerRuntime {
	runtime := internalv1alpha1.ContainerRuntime{}
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
	if ng.Spec.Disruptions.ApprovalMode != "" {
		policy.Mode = string(ng.Spec.Disruptions.ApprovalMode)
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
