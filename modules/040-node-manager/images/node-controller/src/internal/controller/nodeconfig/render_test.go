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
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// TestKeepBootstrapOnlyFields covers the first master's first day-2 render:
// only fields with no answer in the cluster survive the wholesale patch.
// Registry, serverTLSBootstrap and resourceReservation deliberately do NOT.
func TestKeepBootstrapOnlyFields(t *testing.T) {
	bootstrapped := internalv1alpha1.NodeSpec{
		Registry: &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce"},
		// The installer's shape: a selector it chose plus the disk etcd lives
		// on. The mounts are what mark the section as somebody else's — the
		// selector alone is a field this controller renders too.
		Storage: internalv1alpha1.Storage{
			Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
			Mounts: []internalv1alpha1.Mount{{
				Name:              "kubernetes-data",
				PartitionSelector: &internalv1alpha1.PartitionSelector{Size: "10Gi", Blank: true},
				BindTo:            "/var/lib/etcd",
				Mode:              "0700",
			}},
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
			// A selector alone is a field this controller renders itself, so it
			// is re-rendered rather than kept — otherwise the threshold could
			// never be corrected on a node that already exists.
			name: "a selector without mounts is re-rendered",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=2Gi"}}},
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
				Kubelet: internalv1alpha1.Kubelet{MaxPods: 120},
			}

			keepBootstrapOnlyFields(&desired, &tc.existing, tc.reportedIPs)

			require.Equal(t, tc.expStorage, desired.Storage)
			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
			if tc.expNetwork.Hostname != "" {
				require.Equal(t, tc.expNetwork, desired.Network)
			}

			// The cluster owns these, so the render wins even over the bootstrap
			// value; resourceReservation is NodeGroup-owned too (see
			// TestRenderResourceReservation).
			require.Equal(t, rendered, desired.Registry)
			require.Nil(t, desired.Kubelet.ServerTLSBootstrap)
			require.Nil(t, desired.Kubelet.ResourceReservation)

			// Everything else still comes from the render.
			require.Equal(t, "master-0", desired.NodeName)
			require.Equal(t, 120, desired.Kubelet.MaxPods)
		})
	}
}

// Kernel keys are published, not carried over — a carried-over value is frozen
// for the life of the node. The set must match the installer's payload
// (dhctl/pkg/immutable/nodeconfig.go) so the first managed render changes nothing.
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

// Nothing but this carry-over ever writes kubelet.nodeIP, so a re-IPed node
// (DHCP lease, migration, rebuilt VM) used to stay pinned forever to an
// address the cluster no longer routes to it.
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

// The Node watch drops updates that cannot change the rendered spec (kubelet
// heartbeats) but must not drop ones that can: labels are compared whole,
// because the render carries them onto the node's kubelet.
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

// kubeReserved is the group's decision, not the node's: carrying it over meant
// an operator turning the reservation off reached no node at all, and the
// installer's Auto was frozen onto the first master.
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

// Both writers of a platform extension must name the same author, or the first
// master differs from every other node. The literal is spelled out, not taken
// from the constant, so a one-sided change fails here instead of diverging.
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

// The cluster reads a node's cgroup layout off this label and nowhere else;
// without it node-manager raises d8_node_cgroup_v2_unsupported. The render
// publishes it, as the installer does for the first master.
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

// Settings a NodeGroup accepts but the agent's schema does not: passed through,
// they render a NodeConfig the API server rejects, and the node never receives
// one — a tuning setting takes the node out of the cluster instead.
func TestRenderSurvivesSettingsTheAgentSchemaRejects(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:    v1.NodeTypeCloudEphemeral,
			Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeRollingUpdate},
			Kubelet:     &v1.KubeletSpec{MaxPods: ptr.To(int32(1500))},
		},
	}

	// RollingUpdate has no counterpart on the node; the agent knows Manual and
	// Automatic only.
	require.Equal(t, string(v1.DisruptionApprovalModeAutomatic), renderUpdatePolicy(ng).Mode)

	// A NodeGroup takes any number; the agent's schema stops at the ceiling. A
	// ceiling is a better answer than no config at all.
	require.Equal(t, maxPodsCeiling, renderKubelet(ng, &corev1.Node{}, clusterInputs{}).MaxPods)
}

// Values a NodeGroup accepts and the agent's schema refuses field by field. The
// API server rejects the whole object over any one of them, so the group is
// never configured at all — the clamp has to happen here, per field.
func TestRenderNarrowsEveryValueTheAgentSchemaRefuses(t *testing.T) {
	group := func(spec v1.NodeGroupSpec) *v1.NodeGroup {
		spec.NodeType = v1.NodeTypeCloudEphemeral
		return &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: spec}
	}

	t.Run("kubelet", func(t *testing.T) {
		tests := []struct {
			name          string
			kubelet       *v1.KubeletSpec
			expMaxPods    int
			expLogMaxSize string
		}{
			{
				name:          "nothing configured keeps the CRD defaults",
				expMaxPods:    120,
				expLogMaxSize: defaultContainerLogMaxSize,
			},
			{
				// A NodeGroup takes a bare integer; the agent's schema has
				// Minimum=1, and 0 is dropped by omitempty into a silent 120.
				name:          "maxPods 0 is legal for a NodeGroup and refused by the agent",
				kubelet:       &v1.KubeletSpec{MaxPods: ptr.To(int32(0))},
				expMaxPods:    1,
				expLogMaxSize: defaultContainerLogMaxSize,
			},
			{
				name:          "maxPods above the ceiling",
				kubelet:       &v1.KubeletSpec{MaxPods: ptr.To(int32(1500))},
				expMaxPods:    maxPodsCeiling,
				expLogMaxSize: defaultContainerLogMaxSize,
			},
			{
				// The NodeGroup pattern is unanchored, so it matches anything
				// containing "50M"; the agent's CEL rule parses the quantity.
				name:          "containerLogMaxSize with a suffix that is no quantity",
				kubelet:       &v1.KubeletSpec{ContainerLogMaxSize: "50Mib"},
				expMaxPods:    120,
				expLogMaxSize: defaultContainerLogMaxSize,
			},
			{
				name:          "containerLogMaxSize of zero, which the agent refuses as non-positive",
				kubelet:       &v1.KubeletSpec{ContainerLogMaxSize: "0Mi"},
				expMaxPods:    120,
				expLogMaxSize: defaultContainerLogMaxSize,
			},
			{
				name:          "a quantity both schemas accept is passed through",
				kubelet:       &v1.KubeletSpec{ContainerLogMaxSize: "1Gi"},
				expMaxPods:    120,
				expLogMaxSize: "1Gi",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				in := clusterInputs{DefaultMaxPods: 120}

				kubelet := renderKubelet(group(v1.NodeGroupSpec{Kubelet: tt.kubelet}), &corev1.Node{}, in)

				require.Equal(t, tt.expMaxPods, kubelet.MaxPods)
				require.Equal(t, tt.expLogMaxSize, kubelet.ContainerLogMaxSize)
			})
		}
	})

}

// A clamp the operator is not told about leaves the group running something
// nobody configured, so every value the render narrows carries a Warning.
func TestRecordClampedSettings(t *testing.T) {
	tests := []struct {
		name    string
		maxPods *int32
		expects string
	}{
		{name: "a number both schemas accept is not an event", maxPods: ptr.To(int32(250))},
		{name: "above the ceiling", maxPods: ptr.To(int32(1500)), expects: "kubelet.maxPods 1500"},
		{name: "below the floor", maxPods: ptr.To(int32(0)), expects: "kubelet.maxPods 0"},
		{name: "nothing configured", maxPods: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := record.NewFakeRecorder(10)
			r := &Reconciler{}
			r.Recorder = recorder

			r.recordClampedSettings(&v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{Kubelet: &v1.KubeletSpec{MaxPods: tt.maxPods}},
			})
			close(recorder.Events)

			var messages []string
			for event := range recorder.Events {
				messages = append(messages, event)
			}
			if tt.expects == "" {
				require.Empty(t, messages)
				return
			}
			require.Len(t, messages, 1)
			require.Contains(t, messages[0], tt.expects)
			require.Contains(t, messages[0], settingClampedEvent)
		})
	}
}

// Registration taints are rendered for a machine that has not joined yet and
// have no meaning afterwards. Without carrying the field over, every node is
// patched once just to drop it, spending a rollout slot on a no-op.
func TestKeepBootstrapOnlyFieldsKeepsRegistrationTaints(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			NodeTemplate: &v1.NodeTemplate{Taints: []corev1.Taint{
				{Key: "dedicated", Value: "frontend", Effect: corev1.TaintEffectNoSchedule},
			}},
		},
	}

	bootstrap := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{})
	require.NotEmpty(t, bootstrap.Kubelet.RegisterWithTaints, "a machine that has not joined registers with them")

	registered := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", CreationTimestamp: metav1.Now()}}
	dayTwo := renderSpec(ng, registered, clusterInputs{})
	keepBootstrapOnlyFields(&dayTwo, &bootstrap, nil)

	require.Equal(t, bootstrap, dayTwo, "the node it was written for reads as up to date")
}

// The render applies these because the bootstrap file path bypasses API-server
// CRD defaulting. One that drifts from the schema gives a bootstrapping node
// different values than a day-2 one, so they are read from the type, not typed.
func TestRenderDefaultsMatchTheAgentSchema(t *testing.T) {
	markers := kubebuilderMarkers(t, "../../../api/internal.deckhouse.io/v1alpha1/nodeconfig_types.go")

	require.Equal(t, defaultContainerLogMaxSize, markers["ContainerLogMaxSize"]["default"])
	require.Equal(t, strconv.Itoa(defaultContainerLogMaxFiles), markers["ContainerLogMaxFiles"]["default"])
	require.Equal(t, strconv.Itoa(defaultMaxConcurrentDownloads), markers["MaxConcurrentDownloads"]["default"])
	require.Equal(t, strconv.Itoa(maxPodsCeiling), markers["MaxPods"]["validation:Maximum"])
}

// kubebuilderMarkers returns every field's +kubebuilder: markers, keyed by field
// name and then by marker. Field names are unique among the ones read above.
func kubebuilderMarkers(t *testing.T, path string) map[string]map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	require.NoError(t, err)

	markers := map[string]map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok || field.Doc == nil || len(field.Names) != 1 {
			return true
		}
		for _, line := range field.Doc.List {
			marker, ok := strings.CutPrefix(line.Text, "// +kubebuilder:")
			if !ok {
				continue
			}
			name, value, found := strings.Cut(marker, "=")
			if !found {
				continue
			}
			if markers[field.Names[0].Name] == nil {
				markers[field.Names[0].Name] = map[string]string{}
			}
			markers[field.Names[0].Name][name] = strings.Trim(value, `"`)
		}
		return true
	})
	require.NotEmpty(t, markers, "no markers read from %s", path)
	return markers
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
