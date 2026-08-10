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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// TestKeepBootstrapOnlyFields covers what happens to the first master on its
// first day-2 render. Its spec came from a dhctl payload, and only the fields
// below have no answer anywhere in the cluster, so only they survive the
// wholesale patch.
//
// Registry, serverTLSBootstrap and resourceReservation deliberately do NOT: all
// three are rendered from the cluster now. Carrying them over made the master keep whatever the
// bootstrap chose forever — a self-signed kubelet serving certificate with no
// IP in it (so `kubectl exec` and `kubectl logs` failed against every pod on the
// master), and, on the create path where there is no existing object to carry
// anything from, no registry at all.
func TestKeepBootstrapOnlyFields(t *testing.T) {
	bootstrapped := internalv1alpha1.NodeSpec{
		Registry: &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce"},
		Storage: internalv1alpha1.Storage{
			Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
		},
		Kubelet: internalv1alpha1.Kubelet{
			ServerTLSBootstrap:  ptr.To(false),
			NodeIP:              "10.0.0.10",
			ResourceReservation: &internalv1alpha1.ResourceReservation{Mode: "Auto"},
		},
	}

	tests := []struct {
		name        string
		existing    internalv1alpha1.NodeSpec
		reportedIPs []string
		expStorage  internalv1alpha1.Storage
		expNetwork  internalv1alpha1.Network
		expNodeIP   string
	}{
		{
			name:        "bootstrapped master keeps only what the cluster cannot render",
			existing:    bootstrapped,
			reportedIPs: []string{"10.0.0.10"},
			expStorage:  bootstrapped.Storage,
			expNodeIP:   "10.0.0.10",
		},
		{
			name:       "worker with nothing to keep is left as rendered",
			existing:   internalv1alpha1.NodeSpec{},
			expStorage: internalv1alpha1.Storage{},
		},
		{
			// The shape a node bootstrapped before this controller stopped
			// fabricating selectors carries on its CONFIG partition. It is not
			// explicit — it names no attribute — so the rendered empty storage
			// replaces it and the node heals itself on the next sync.
			name: "a selector that constrains nothing is not kept",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{}}},
			},
			expStorage: internalv1alpha1.Storage{},
		},
		{
			// A statically addressed machine: the render must not put it back
			// on DHCP — the OS renders its network from this spec on every
			// boot, and eth0-DHCP over a static address is a dead node.
			name: "a static network survives the render, the hostname does not",
			existing: internalv1alpha1.NodeSpec{
				Network: internalv1alpha1.Network{
					Hostname: "what-the-machine-said",
					Interfaces: []internalv1alpha1.NetworkInterface{{
						Name: "eth0", DHCP: false, Addresses: []string{"192.0.2.10/24"}, Gateway: "192.0.2.1",
					}},
				},
			},
			expStorage: internalv1alpha1.Storage{},
			expNetwork: internalv1alpha1.Network{
				Hostname: "master-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{
					Name: "eth0", DHCP: false, Addresses: []string{"192.0.2.10/24"}, Gateway: "192.0.2.1",
				}},
			},
		},
		{
			name: "the rendered DHCP default is not explicit and is re-rendered",
			existing: internalv1alpha1.NodeSpec{
				Network: internalv1alpha1.Network{
					Hostname:   "master-0",
					Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
				},
			},
			expStorage: internalv1alpha1.Storage{},
			expNetwork: internalv1alpha1.Network{Hostname: "master-0", Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}}},
		},
		{
			name: "an explicit device is as specific as a selector",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
			},
			expStorage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
		},
		{
			// The disk etcd lives on is given to a control-plane node this way.
			// Dropping the list would leave nothing under /var/lib/etcd on the next
			// pass, and etcd would come up as a brand new cluster after a reboot.
			name: "the mounts a master keeps etcd on are not dropped",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Mounts: []internalv1alpha1.Mount{{
					Name:              "kubernetes-data",
					PartitionSelector: &internalv1alpha1.PartitionSelector{Size: "10Gi", Blank: true},
					BindTo:            "/var/lib/etcd",
					Mode:              "0700",
				}}},
			},
			expStorage: internalv1alpha1.Storage{Mounts: []internalv1alpha1.Mount{{
				Name:              "kubernetes-data",
				PartitionSelector: &internalv1alpha1.PartitionSelector{Size: "10Gi", Blank: true},
				BindTo:            "/var/lib/etcd",
				Mode:              "0700",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered := &internalv1alpha1.Registry{Address: "rendered.example.com"}
			desired := internalv1alpha1.NodeSpec{
				NodeName: "master-0",
				Registry: rendered,
				Network: internalv1alpha1.Network{
					Hostname:   "master-0",
					Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
				},
				Kubelet: internalv1alpha1.Kubelet{MaxPods: maxPodsPerNodeCIDR24},
			}

			keepBootstrapOnlyFields(&desired, &tc.existing, tc.reportedIPs)

			require.Equal(t, tc.expStorage, desired.Storage)
			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
			if tc.expNetwork.Hostname != "" {
				require.Equal(t, tc.expNetwork, desired.Network)
			}

			// The cluster owns these, so the render wins even when the node was
			// bootstrapped with something else. resourceReservation is one of
			// them: the NodeGroup has that field, so an operator changing it has
			// to reach the nodes (see TestRenderResourceReservation).
			require.Equal(t, rendered, desired.Registry)
			require.Nil(t, desired.Kubelet.ServerTLSBootstrap)
			require.Nil(t, desired.Kubelet.ResourceReservation)

			// Everything else still comes from the render.
			require.Equal(t, "master-0", desired.NodeName)
			require.Equal(t, maxPodsPerNodeCIDR24, desired.Kubelet.MaxPods)
		})
	}
}

// The kernel keys a node must keep running are published, not carried over from
// whatever the node already has. Carrying them over cannot be undone: after the
// first patch the object's own spec holds them, so nothing can tell a value the
// installer chose from one an earlier pass copied, and the value is frozen for
// the life of the node. Publishing them keeps the set the same on every
// immutable node and the same as the payload the installer writes for the first
// master (dhctl/pkg/immutable/nodeconfig.go), so that master's first managed
// render changes nothing about its kernel.
func TestRenderKernelPublishesEveryKeyANodeMustKeep(t *testing.T) {
	sysctl := renderKernel().Sysctl

	require.Equal(t, map[string]internalv1alpha1.SysctlValue{
		"kernel.panic":         "10",
		"kernel.panic_on_oops": "1",
		"net.ipv4.ip_forward":  "1",
		"vm.max_map_count":     "262144",
	}, sysctl)

	// And a node that booted with a different value for one of them is brought
	// back to the cluster's, rather than keeping its own forever.
	stale := internalv1alpha1.NodeSpec{
		Kernel: internalv1alpha1.Kernel{Sysctl: map[string]internalv1alpha1.SysctlValue{"kernel.panic": "0"}},
	}
	desired := internalv1alpha1.NodeSpec{Kernel: renderKernel()}
	keepBootstrapOnlyFields(&desired, &stale, nil)
	require.Equal(t, internalv1alpha1.SysctlValue("10"), desired.Kernel.Sysctl["kernel.panic"])
}

// Nothing but this carry-over ever writes kubelet.nodeIP, so a node that is
// re-IPed — a new DHCP lease, a migration, a rebuilt VM — used to stay pinned
// to the address it first had, registering forever under an address the cluster
// no longer routes to it.
func TestKeepBootstrapOnlyFieldsReleasesAStaleNodeIP(t *testing.T) {
	tests := []struct {
		name        string
		reportedIPs []string
		expNodeIP   string
	}{
		{
			name:        "the node still reports the address it was given",
			reportedIPs: []string{"10.0.0.10"},
			expNodeIP:   "10.0.0.10",
		},
		{
			name:        "the node reports a different address now",
			reportedIPs: []string{"10.0.0.77"},
			expNodeIP:   "",
		},
		{
			name:        "the node reports several, one of them the pinned one",
			reportedIPs: []string{"10.0.0.77", "10.0.0.10"},
			expNodeIP:   "10.0.0.10",
		},
		{
			name:      "the node reports no address at all, so nothing contradicts it",
			expNodeIP: "10.0.0.10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			existing := internalv1alpha1.NodeSpec{Kubelet: internalv1alpha1.Kubelet{NodeIP: "10.0.0.10"}}
			desired := internalv1alpha1.NodeSpec{}

			keepBootstrapOnlyFields(&desired, &existing, tc.reportedIPs)

			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
		})
	}
}

// kubelet exits when --node-labels carries a label it may not accept, so a node
// whose group template names one comes up with no kubelet at all. The master
// group names two of them.
func TestRenderNodeLabelsDropsWhatKubeletRefuses(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudPermanent,
			NodeTemplate: &v1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
					"node.deckhouse.io/group":               "master",
					"topology.kubernetes.io/zone":           "eu-1a",
					"node.kubernetes.io/instance-type":      "m1",
					"example.com/team":                      "candi",
				},
			},
		},
	}

	labels := renderNodeLabels(ng)

	for _, refused := range []string{"node-role.kubernetes.io/control-plane", "node-role.kubernetes.io/master"} {
		_, present := labels[refused]
		require.False(t, present, "%s must not reach --node-labels", refused)
	}
	for _, kept := range []string{"node.deckhouse.io/group", "topology.kubernetes.io/zone", "node.kubernetes.io/instance-type", "example.com/team"} {
		require.Contains(t, labels, kept)
	}
}

// The Node watch drops updates that cannot change the rendered spec — a fleet's
// worth of kubelet heartbeats, which is most of what arrives here. It must not
// drop the ones that can: everything the render reads off a Node lives in the
// object's metadata, and a NodeExtensionRequest can select on any label at all,
// so labels are compared whole rather than by the two names this package knows.
func TestNodeRenderInputsChanged(t *testing.T) {
	node := func(mutate func(*corev1.Node)) *corev1.Node {
		n := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "worker-0",
				UID:               "uid-1",
				CreationTimestamp: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				Labels: map[string]string{
					nodecommon.NodeGroupLabel:     "workers",
					"topology.kubernetes.io/zone": "eu-1a",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{
					Type:              corev1.NodeReady,
					Status:            corev1.ConditionTrue,
					LastHeartbeatTime: metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
				}},
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.10"}},
			},
		}
		if mutate != nil {
			mutate(n)
		}
		return n
	}

	tests := []struct {
		name       string
		after      *corev1.Node
		expChanged bool
	}{
		{
			name: "a kubelet heartbeat",
			after: node(func(n *corev1.Node) {
				n.Status.Conditions[0].LastHeartbeatTime = metav1.NewTime(time.Date(2026, 1, 2, 3, 5, 5, 0, time.UTC))
			}),
			expChanged: false,
		},
		{
			name: "the node reports a new address",
			// Read from the API server when it is needed, and stripped from the
			// cached object, so it cannot be seen here either way.
			after: node(func(n *corev1.Node) {
				n.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.77"}}
			}),
			expChanged: false,
		},
		{
			name:       "the node is cordoned",
			after:      node(func(n *corev1.Node) { n.Spec.Unschedulable = true }),
			expChanged: false,
		},
		{
			name:       "the node moves to another group",
			after:      node(func(n *corev1.Node) { n.Labels[nodecommon.NodeGroupLabel] = "masters" }),
			expChanged: true,
		},
		{
			name:       "the node leaves its group",
			after:      node(func(n *corev1.Node) { delete(n.Labels, nodecommon.NodeGroupLabel) }),
			expChanged: true,
		},
		{
			name:       "the node becomes a control-plane node",
			after:      node(func(n *corev1.Node) { n.Labels[controlPlaneRoleLabel] = "" }),
			expChanged: true,
		},
		{
			name:       "a label nothing here names, which an extension request may select on",
			after:      node(func(n *corev1.Node) { n.Labels["storage.deckhouse.io/drbd"] = "true" }),
			expChanged: true,
		},
		{
			name:       "the object was replaced",
			after:      node(func(n *corev1.Node) { n.UID = "uid-2" }),
			expChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expChanged, nodeRenderInputsChanged(node(nil), tt.after))
		})
	}

	// An event missing a side is passed through rather than guessed at.
	require.True(t, nodeRenderInputsChanged(nil, node(nil)))
	require.True(t, nodeRenderInputsChanged(node(nil), nil))
}

// kubeReserved is the group's decision, not the node's. It used to be carried
// over from whatever the node already carried, so an operator turning the
// reservation off on an immutable group reached no node at all — no change, no
// event, no log line — and the installer's Auto was frozen onto the first
// master, where after the first patch nothing could tell it from a value an
// earlier pass had copied.
func TestRenderResourceReservation(t *testing.T) {
	tests := []struct {
		name    string
		kubelet *v1.KubeletSpec
		expMode string
	}{
		{
			name:    "a group that says nothing reserves by capacity",
			expMode: resourceReservationModeAuto,
		},
		{
			name:    "the operator turns the reservation off",
			kubelet: &v1.KubeletSpec{ResourceReservation: &v1.ResourceReservationSpec{Mode: "Off"}},
			expMode: "Off",
		},
		{
			// A NodeGroup can name exact amounts; an immutable node reserves by
			// capacity or not at all, and reserving something is nearer to what
			// was asked for than reserving nothing.
			name: "exact amounts are clamped to reserving by capacity",
			kubelet: &v1.KubeletSpec{ResourceReservation: &v1.ResourceReservationSpec{
				Mode:   resourceReservationModeStatic,
				Static: &v1.StaticResourceReservation{},
			}},
			expMode: resourceReservationModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, Kubelet: tt.kubelet}}

			reservation := renderResourceReservation(ng)

			require.NotNil(t, reservation)
			require.Equal(t, tt.expMode, reservation.Mode)
			// And it reaches the node, rather than being computed and dropped on
			// the way into the spec.
			require.Equal(t, reservation, renderKubelet(ng, &corev1.Node{}, clusterInputs{}).ResourceReservation)
		})
	}
}

// Both writers of a platform extension have to name the same author, or the
// first master differs from every other node on three entries of an array that
// is compared whole. The literal is spelled out here rather than taken from the
// constant, so that changing it on this side alone fails instead of silently
// re-opening the divergence with dhctl (osImageNameAndTag's neighbour in
// dhctl/pkg/immutable/nodeconfig.go).
func TestRenderExtensionsNamesTheModuleThatAskedForThem(t *testing.T) {
	extensions := renderExtensions(map[string]string{
		containerdExtension: "sha256:1",
		kubeletExtension:    "sha256:2",
		cniExtension:        "sha256:3",
	})

	require.Len(t, extensions, 3)
	for _, ext := range extensions {
		require.Equal(t, "node-manager", ext.RequestedBy, "%s must name the module both writers belong to", ext.Name)
	}
}

// The cluster reads the cgroup layout of a node off this label and nowhere else:
// without it node-manager counts the node as unable to run containerd v2 and
// raises d8_node_cgroup_v2_unsupported for it. It cannot come from the group's
// template — the master's template carries only labels kubelet refuses — so the
// render publishes it, as the installer does for the first master.
func TestRenderNodeLabelsPublishesTheCgroupLayout(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
	}

	labels := renderNodeLabels(ng)

	require.Equal(t, internalv1alpha1.NodeLabelValue(cgroupV2Value), labels[cgroupLabel])
	// And kubelet is allowed to set it, or the node would exit instead of
	// registering.
	require.True(t, kubeletMaySetLabel(cgroupLabel))
}

// The cluster publishes policy, not tuning. The tuning lives in the OS image,
// which knows its own kernel — rendering it from here meant sending one set to
// nodes whose kernels may differ, and one missing knob failed a node's entire
// configuration pass.
func TestRenderKernelCarriesOnlyClusterPolicy(t *testing.T) {
	sysctl := renderKernel().Sysctl

	// kubelet refuses to start without these, and fencing changes the delay.
	require.Equal(t, internalv1alpha1.SysctlValue("10"), sysctl["kernel.panic"])
	require.Equal(t, internalv1alpha1.SysctlValue("1"), sysctl["kernel.panic_on_oops"])

	// Everything the image owns must not be published from here.
	for _, key := range []string{
		"fs.inotify.max_user_watches",
		"kernel.pid_max",
		"net.ipv4.neigh.default.gc_thresh3",
		"net.ipv4.conf.all.rp_filter",
		"vm.min_free_kbytes",
		// Derived from the node's core count; the agent computes it.
		"net.netfilter.nf_conntrack_max",
	} {
		require.NotContains(t, sysctl, key)
	}
}

// Settings a NodeGroup accepts but the agent's schema does not. Passing them
// through renders a NodeConfig the API server rejects, and a node whose config
// is rejected never receives one — so a setting meant to tune the node takes it
// out of the cluster instead.
func TestRenderSurvivesSettingsTheAgentSchemaRejects(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:    v1.NodeTypeCloudEphemeral,
			Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeRollingUpdate},
			Kubelet:     &v1.KubeletSpec{MaxPods: ptr.To(int32(1000))},
		},
	}

	// RollingUpdate has no counterpart on the node; the agent knows Manual and
	// Automatic only.
	require.Equal(t, string(v1.DisruptionApprovalModeAutomatic), renderUpdatePolicy(ng).Mode)

	// The documentation suggests 1000 for a /21 pod subnet; the schema stops at
	// 500. A ceiling is a better answer than no config at all.
	require.Equal(t, maxPodsCeiling, renderKubelet(ng, &corev1.Node{}, clusterInputs{}).MaxPods)
}

// kubelet compares label namespaces by suffix. Equality here dropped legal
// labels silently, and let through two that do not exist upstream at all —
// which kubelet refuses, taking the node down with it.
func TestKubeletMaySetLabelFollowsKubeletsOwnRule(t *testing.T) {
	allowed := []string{
		"node.deckhouse.io/group",
		"example.com/team",
		"topology.kubernetes.io/zone",
		"node.kubernetes.io/instance-type",
		// Under an allowed prefix: kubelet accepts it, so must we.
		"myprefix.node.kubernetes.io/foo",
		"myprefix.kubelet.kubernetes.io/bar",
	}
	for _, key := range allowed {
		require.True(t, kubeletMaySetLabel(key), "kubelet accepts %s", key)
	}

	refused := []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		"kubernetes.io/anything-else",
		// No GA spelling of these exists; kubelet exits on them.
		"failure-domain.kubernetes.io/zone",
		"failure-domain.kubernetes.io/region",
	}
	for _, key := range refused {
		require.False(t, kubeletMaySetLabel(key), "kubelet refuses %s", key)
	}
}
