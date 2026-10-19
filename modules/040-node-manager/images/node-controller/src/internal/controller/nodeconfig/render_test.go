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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The machine owns what it wrote when it registered: the apply set of an existing
// object carries none of it, and the comparison that decides a write ignores it. The
// bootstrap token is not the machine's: the apply leaves it out, so a copied one is removed.
func TestTheApplySetLeavesTheMachineFieldsOut(t *testing.T) {
	registered := internalv1alpha1.NodeSpec{
		NodeName:    "master-0",
		StatusToken: "e3b0c44298fc1c14",
		Storage: internalv1alpha1.Storage{
			Disk:   internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
			Mounts: []internalv1alpha1.Mount{{Name: "kubernetes-data", BindTo: "/var/lib/etcd"}},
		},
		Network: internalv1alpha1.Network{
			Hostname:   "master-0",
			Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.10/24"}}},
		},
		Kubelet: internalv1alpha1.Kubelet{
			MaxPods:            110,
			RegisterWithTaints: []internalv1alpha1.Taint{{Key: "dedicated", Effect: "NoSchedule"}},
		},
	}

	applied := withoutMachineFields(registered)

	require.Equal(t, internalv1alpha1.NodeSpec{
		NodeName: "master-0",
		Kubelet:  internalv1alpha1.Kubelet{MaxPods: 110},
	}, applied)
	require.NotEmpty(t, registered.Storage.Mounts, "the spec it was taken from is not changed")

	rendered := &internalv1alpha1.NodeConfig{Spec: renderSpecOf(applied)}
	require.True(t, specUpToDate(&internalv1alpha1.NodeConfig{Spec: registered}, rendered),
		"a node keeping what it registered with is not drift")

	withToken := registered
	withToken.Kubelet.BootstrapToken = "abcdef.0123456789abcdef"
	require.Empty(t, withoutMachineFields(withToken).Kubelet.BootstrapToken, "the apply never carries the token")
	require.False(t, specUpToDate(&internalv1alpha1.NodeConfig{Spec: withToken}, rendered),
		"a token copied into the object is a change the apply removes")
}

func renderSpecOf(spec internalv1alpha1.NodeSpec) internalv1alpha1.NodeSpec {
	spec.Network = internalv1alpha1.Network{Hostname: spec.NodeName}
	spec.Storage = internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: systemDiskSelectorSize}}}
	return spec
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
	// back to the cluster's: kernel is not the machine's, so it is applied.
	stale := internalv1alpha1.NodeSpec{
		Kernel: internalv1alpha1.Kernel{Sysctl: map[string]internalv1alpha1.SysctlValue{"kernel.panic": "0"}},
	}
	require.False(t, specUpToDate(&internalv1alpha1.NodeConfig{Spec: stale},
		&internalv1alpha1.NodeConfig{Spec: internalv1alpha1.NodeSpec{Kernel: renderKernel()}}))
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
// because a NodeExtensionRequest can select on any label at all.
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
			// The node picks its own address from spec.internalNetworkCIDRs, so
			// nothing here reads one; the cache strips them anyway.
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

// A platform extension names no requester: the node tells a request from the
// platform's own by requestedBy, and dhctl writes the first master the same way.
func TestRenderExtensionsNameNoRequester(t *testing.T) {
	extensions := renderExtensions(map[string]string{
		containerdExtension: "sha256:1",
		kubeletExtension:    "sha256:2",
		cniExtension:        "sha256:3",
	})

	require.Len(t, extensions, 3)
	for _, ext := range extensions {
		require.Nil(t, ext.RequestedBy, "%s is the platform's own", ext.Name)
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
				// maxPods follows the pod subnet; the log size is left to the node.
				name:       "nothing configured leaves the defaults to the node",
				expMaxPods: 120,
			},
			{
				// A NodeGroup takes a bare integer; the agent's schema has
				// Minimum=1, and 0 is dropped by omitempty into a silent 120.
				name:       "maxPods 0 is legal for a NodeGroup and refused by the agent",
				kubelet:    &v1.KubeletSpec{MaxPods: ptr.To(int32(0))},
				expMaxPods: 1,
			},
			{
				name:       "maxPods above the ceiling",
				kubelet:    &v1.KubeletSpec{MaxPods: ptr.To(int32(1500))},
				expMaxPods: maxPodsCeiling,
			},
			{
				// The NodeGroup pattern is unanchored, so it matches anything
				// containing "50M"; the agent's CEL rule parses the quantity.
				name:       "containerLogMaxSize with a suffix that is no quantity",
				kubelet:    &v1.KubeletSpec{ContainerLogMaxSize: "50Mib"},
				expMaxPods: 120,
			},
			{
				name:       "containerLogMaxSize of zero, which the agent refuses as non-positive",
				kubelet:    &v1.KubeletSpec{ContainerLogMaxSize: "0Mi"},
				expMaxPods: 120,
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
				Spec: v1.NodeGroupSpec{
					SystemType: v1.SystemTypeImmutable,
					Kubelet:    &v1.KubeletSpec{MaxPods: tt.maxPods},
				},
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

// Registration taints are rendered for a machine that has not joined yet and have no
// meaning afterwards. They are the machine's once it registered, so a joined node is
// not patched just to drop them.
func TestARegisteredNodeIsNotPatchedToDropItsRegistrationTaints(t *testing.T) {
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

	require.True(t, specUpToDate(&internalv1alpha1.NodeConfig{Spec: bootstrap}, &internalv1alpha1.NodeConfig{Spec: dayTwo}),
		"the node it was written for reads as up to date")
}

// The value of an unset field lives in nodelet alone (FREEZE-SPEC 1.6): a default in
// the schema is a second copy, and the API server would write it into every object.
func TestTheShippedCRDDefaultsNothing(t *testing.T) {
	var defaults []string
	var walk func(path string, node any)
	walk = func(path string, node any) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "default" {
					defaults = append(defaults, path)
				}
				walk(path+"."+key, child)
			}
		case []any:
			for _, child := range value {
				walk(path+"[]", child)
			}
		}
	}
	walk("", nodeConfigCRDSchema(t))
	require.Empty(t, defaults)
}

// The shipped CRD is what the API server enforces; the render clamps to the same
// bounds, and a value beyond them is refused whole.
func TestRenderBoundsMatchTheShippedCRD(t *testing.T) {
	schema := nodeConfigCRDSchema(t)

	require.Equal(t, float64(maxPodsCeiling), crdField(t, schema, "spec", "kubelet", "maxPods")["maximum"])

	// No default: every writer names the type, and the API server refuses one that does not.
	require.Contains(t, crdField(t, schema, "spec")["required"], "systemType")

	// The render is what fills the list, so the bound it caps at is the bound the
	// API server enforces: a longer list is refused whole and the node is left
	// without a config at all.
	require.Equal(t, float64(maxStaticPods), crdField(t, schema, "spec", "staticPods")["maxItems"])
}

// nodeConfigCRDSchema reads the NodeConfig CRD the module ships, located the way
// the envtest suites locate it, and returns its single version's schema.
func nodeConfigCRDSchema(t *testing.T) map[string]any {
	t.Helper()
	return testenv.ShippedCRDSchema(t, testenv.NodeConfigCRDFile)
}

// crdField walks the schema down a property path to one field.
func crdField(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()

	node := schema
	for _, name := range path {
		properties, ok := node["properties"].(map[string]any)
		require.True(t, ok, "nothing above %s has properties", name)
		node, ok = properties[name].(map[string]any)
		require.True(t, ok, "the CRD has no %s", name)
	}
	return node
}

// A NodeGroup leaves a taint's effect optional (crds/node_group.yaml:1944) and
// kubelet cannot register a node with an effectless taint — it exits, and the
// node never joins. Such a taint is dropped and reported instead.
func TestRenderTaintsDropsWhatKubeletCannotRegister(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			SystemType: v1.SystemTypeImmutable,
			NodeTemplate: &v1.NodeTemplate{Taints: []corev1.Taint{
				{Key: "dedicated", Value: "frontend"},
				{Key: "ship-class", Value: "frigate", Effect: corev1.TaintEffectNoExecute},
			}},
		},
	}

	// A machine that has not joined yet: registration taints are only rendered
	// for one.
	spec := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{})
	require.Equal(t, []internalv1alpha1.Taint{
		{Key: "ship-class", Value: "frigate", Effect: string(corev1.TaintEffectNoExecute)},
	}, spec.Kubelet.RegisterWithTaints)

	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{}
	r.Recorder = recorder
	r.recordClampedSettings(ng)
	close(recorder.Events)

	var messages []string
	for event := range recorder.Events {
		messages = append(messages, event)
	}
	require.Len(t, messages, 1, "the operator has to be told which taint the nodes do not get")
	require.Contains(t, messages[0], "dedicated")
	require.Contains(t, messages[0], settingClampedEvent)
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

// A value legal for a NodeGroup must stay legal once copied into a NodeConfig:
// the API server rejects the whole object over any one field, and the group is
// then left with no configuration at all. Cases come from crds/node_group.yaml.
func TestRenderedSpecPassesTheAgentSchema(t *testing.T) {
	c := nodeConfigAPIClient(t)

	labels := map[string]string{
		// Neither a qualified name nor within the length the CEL rules asked
		// for, and a NodeGroup takes it.
		"example.com/" + strings.Repeat("a", 320): "long-key",
	}
	for i := range 100 {
		labels[fmt.Sprintf("example.com/label-%d", i)] = strings.Repeat("v", 63)
	}
	// Longer than a Kubernetes label value may be, and a NodeGroup takes it.
	labels["example.com/long-value"] = strings.Repeat("v", 100)

	tests := []struct {
		name string
		spec v1.NodeGroupSpec
		// patch carries a value omitempty drops out of the render, so the
		// schema is still asked to accept it.
		patch string
	}{
		{
			// crds/node_group.yaml:1638: a bare integer, and 0 is containerd's "no limit".
			name:  "maxConcurrentDownloads 0",
			spec:  v1.NodeGroupSpec{CRI: &v1.CRISpec{Containerd: &v1.ContainerdSpec{MaxConcurrentDownloads: ptr.To(0)}}},
			patch: `{"spec":{"containerRuntime":{"maxConcurrentDownloads":0}}}`,
		},
		{
			// crds/node_group.yaml:432: the hour may be a single digit.
			name: "update window with a one-digit hour",
			spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{Automatic: &v1.AutomaticDisruptionSpec{
				Windows: []v1.DisruptionWindow{{From: "8:00", To: "9:30", Days: []string{"Mon"}}},
			}}},
		},
		{
			// crds/node_group.yaml:1944: key and value are bare strings. A taint
			// with no effect is not here — it never reaches the object, kubelet
			// having no way to register one (TestRenderTaintsDropsWhatKubeletCannotRegister).
			name: "taint with a long key and value",
			spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{Taints: []corev1.Taint{
				{
					Key:    "dedicated.example.com/" + strings.Repeat("a", 300),
					Value:  strings.Repeat("b", 80),
					Effect: corev1.TaintEffectNoSchedule,
				},
			}}},
		},
		{
			// crds/node_group.yaml:1915: additionalProperties, no cap on how many.
			name: "a hundred node labels",
			spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{Labels: labels}},
		},
	}

	in := clusterInputs{
		OSImage:        internalv1alpha1.OSImage{Digest: "sha256:" + strings.Repeat("a", 64)},
		DefaultMaxPods: 120,
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.spec.NodeType = v1.NodeTypeCloudEphemeral
			ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: tt.spec}
			// A machine that has not joined yet: registration taints are only
			// rendered for one.
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("worker-%d", i)}}

			config := &internalv1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: node.Name},
				Spec:       renderSpec(ng, node, in),
			}
			require.NoError(t, c.Create(context.Background(), config), "the API server refused a spec rendered from a legal NodeGroup")

			if tt.patch == "" {
				return
			}
			require.NoError(t, c.Patch(context.Background(), config, client.RawPatch(types.MergePatchType, []byte(tt.patch))))
		})
	}
}

// An explicit zero is containerd's "no limit"; it has to reach the object as a
// zero, not vanish behind omitempty and come back as the CRD default.
func TestAnExplicitZeroDownloadsReachesTheAPI(t *testing.T) {
	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{CRI: &v1.CRISpec{Containerd: &v1.ContainerdSpec{MaxConcurrentDownloads: ptr.To(0)}}}}
	data, err := json.Marshal(renderContainerRuntime(ng, clusterInputs{}, nil))
	require.NoError(t, err)
	require.Contains(t, string(data), `"maxConcurrentDownloads":0`)
}

// crdItems steps into an array's item schema, which crdField cannot: items is a
// sibling of properties, not one of them.
func crdItems(t *testing.T, field map[string]any) map[string]any {
	t.Helper()

	items, ok := field["items"].(map[string]any)
	require.True(t, ok, "the field is not an array")
	return items
}

// The static-pod contract lives in two repositories — this one and nodelet's
// internal/config/types.go — and the shipped CRD is what the API server enforces
// on the objects this controller writes. A field that never reached the schema is
// a field the API server strips on the way in, silently.
func TestShippedCRDCarriesTheStaticPodContract(t *testing.T) {
	schema := nodeConfigCRDSchema(t)

	// The bounds are one number: 16 manifests of 32 KiB is 512 KiB, which has to
	// leave room under etcd's 1.5 MiB request limit for the rest of the spec.
	staticPods := crdField(t, schema, "spec", "staticPods")
	require.Equal(t, "map", staticPods["x-kubernetes-list-type"])
	require.Equal(t, []any{"name"}, staticPods["x-kubernetes-list-map-keys"])
	require.Equal(t, float64(maxStaticPods), staticPods["maxItems"])

	manifest := crdField(t, crdItems(t, staticPods), "manifest")
	require.Equal(t, float64(1), manifest["minLength"])
	require.Equal(t, float64(32768), manifest["maxLength"])

	// Who writes containerd's registry.d. Unset is Nodelet, which keeps a cluster
	// that never heard of an agent behaving exactly as it did before.
	owner := crdField(t, schema, "spec", "containerRuntime", "registryConfigOwner")
	require.NotContains(t, owner, "default")
	require.Equal(t, []any{string(internalv1alpha1.RegistryConfigOwnerNodelet), string(internalv1alpha1.RegistryConfigOwnerRegistryAgent)}, owner["enum"])

	// Two states, not three: a node either holds the file the spec asked for or
	// it does not. What an operator does next is in the reason, because the three
	// causes call for three different actions — and an enum is what keeps the two
	// repositories from drifting into four.
	podStatus := crdItems(t, crdField(t, schema, "status", "staticPods"))
	require.Equal(t, []any{staticPodStateWritten, staticPodStateFailed},
		crdField(t, podStatus, "state")["enum"],
		"the roll-up reads these two spellings and nothing else")
	require.Equal(t, []any{"ManifestRejected", "WriteFailed", "RemoveFailed"},
		crdField(t, podStatus, "reason")["enum"])
}

// pause ships in the containerd extension, which imports it itself, so the
// render names no sandbox image and preloads nothing: nodelet parses strictly,
// and a sandboxImage here would override the one the extension imported.
func TestRenderContainerRuntimeLeavesImagesToTheExtension(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}

	for _, tt := range []struct {
		name      string
		agentMode bool
		owner     any
	}{
		{name: "no agent"},
		{name: "agent mode", agentMode: true, owner: string(internalv1alpha1.RegistryConfigOwnerRegistryAgent)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := clusterInputs{RegistryAgentMode: tt.agentMode}
			if tt.agentMode {
				in.NodeStaticPodRequests = withAgentStaticPod()
			}
			spec := renderSpec(ng, node, in)

			data, err := json.Marshal(spec)
			require.NoError(t, err)
			var raw map[string]any
			require.NoError(t, json.Unmarshal(data, &raw))
			// Nothing to say leaves the section out, as the API sends it.
			containerRuntime, _ := raw["containerRuntime"].(map[string]any)
			require.NotContains(t, containerRuntime, "sandboxImage")
			require.NotContains(t, containerRuntime, "localImages")
			require.Equal(t, tt.owner, containerRuntime["registryConfigOwner"])
		})
	}
}

// The render leaves the sandbox to the node, and the node names the pause the
// Engine containerd unit imports on start. That import has to be the one the
// bashible unit does, line for line, from a tar the sysext puts at that path.
func TestTheEngineContainerdUnitImportsPauseLikeBashible(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		paths := testenv.ModuleCRDPaths(path)
		require.Len(t, paths, 1)
		raw, err := os.ReadFile(paths[0])
		require.NoError(t, err)
		return string(raw)
	}
	pauseLines := func(unit string) []string {
		var lines []string
		for _, line := range strings.Split(unit, "\n") {
			if strings.HasPrefix(line, "ExecStartPost=") && strings.Contains(line, "pause") {
				lines = append(lines, line)
			}
		}
		return lines
	}

	const dir = "../images/platform/registrypackages/containerd/"
	bashible := pauseLines(read(dir + "scripts/containerd.service"))
	require.NotEmpty(t, bashible, "the bashible unit no longer imports pause")
	require.Equal(t, bashible, pauseLines(read(dir+"sysext/sysext-containerd.service")))
	require.Contains(t, read(dir+"werf.inc.yaml"), "to: /sysext/opt/deckhouse/images\n")
}

// With a CRD default the API server would fill sandboxImage in on every object
// the render leaves it out of, and the node would never see it empty.
func TestShippedCRDLeavesSandboxImageEmpty(t *testing.T) {
	schema := nodeConfigCRDSchema(t)
	require.NotContains(t, crdField(t, schema, "spec", "containerRuntime", "sandboxImage"), "default")
	require.NotContains(t, crdField(t, schema, "spec", "containerRuntime")["properties"], "localImages")
	require.NotContains(t, crdField(t, schema, "status")["properties"], "localImages")
}

// The whole point of the object: what a module published reaches the node's
// spec, byte for byte, and nothing of the objects meant for another group.
func TestRenderSpecCompilesStaticPods(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}

	ordered := orderedNSPRs([]deckhousev1alpha1.NodeStaticPodRequest{
		nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{}),
		nspr("other-group-only", deckhousev1alpha1.NodeStaticPodRequestSpec{
			NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"storage"}},
		}),
	})

	spec := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{
		NodeStaticPodRequests:         ordered,
		NodeStaticPodRequestsRejected: refusedEverywhere(ordered),
	})

	require.Len(t, spec.StaticPods, 1)
	require.Equal(t, "registry-agent", spec.StaticPods[0].Name)
	require.Equal(t, podManifest("registry-agent"), spec.StaticPods[0].Manifest,
		"the manifest reaches the node byte for byte")

}

// A spec holding more pods than the schema accepts is refused whole, which
// leaves the node with no config at all — so the render drops the surplus
// instead, and drops it in the order the contest settled: the object that
// arrived last loses its place, whatever it is called.
func TestRenderSpecBoundsStaticPods(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}
	created := metav1.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	nsprs := make([]deckhousev1alpha1.NodeStaticPodRequest, 0, maxStaticPods+1)
	for i := range maxStaticPods {
		nsprs = append(nsprs, nsprCreated(fmt.Sprintf("pod-%02d", i),
			metav1.NewTime(created.Add(time.Duration(i)*time.Minute)),
			deckhousev1alpha1.NodeStaticPodRequestSpec{}))
	}
	// Named so that it sorts first and was created last: a cap taken after the
	// render sorts by name would keep this one and drop somebody older.
	nsprs = append(nsprs, nsprCreated("aaa-newest",
		metav1.NewTime(created.Add(time.Hour)), deckhousev1alpha1.NodeStaticPodRequestSpec{}))
	ordered := orderedNSPRs(nsprs)

	spec := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{
		NodeStaticPodRequests:         ordered,
		NodeStaticPodRequestsRejected: refusedEverywhere(ordered),
	})

	require.Len(t, spec.StaticPods, maxStaticPods)
	names := make([]string, 0, len(spec.StaticPods))
	for _, pod := range spec.StaticPods {
		names = append(names, pod.Name)
	}
	require.NotContains(t, names, "aaa-newest", "the object that arrived last is the one that loses its place")
	require.Contains(t, names, fmt.Sprintf("pod-%02d", maxStaticPods-1), "and no older object is dropped in its stead")
}

// withAgentStaticPod is the registry module having published its agent, in the
// shape renderSpec reads it.
func withAgentStaticPod() []*deckhousev1alpha1.NodeStaticPodRequest {
	return orderedNSPRs([]deckhousev1alpha1.NodeStaticPodRequest{
		nspr(registryAgentStaticPodName, deckhousev1alpha1.NodeStaticPodRequestSpec{}),
	})
}

// Who writes containerd's registry.d is a field, not a detection: a node that
// worked it out from "somebody wrote _default" would behave differently
// depending on which of two writers ran first.
//
// It takes both halves. The registry module saying it owns the pull path is an
// intent about the cluster; whether this node can act on it is a fact about the
// node, and on an Immutable group — the only kind that has a node config — the
// agent arrives as a static pod or not at all. Acting on the intent alone strands
// the node: registry.d released, no agent to write one, and no way to pull the
// agent that would.
func TestRenderRegistryOwner(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}

	// Unset is the node's Nodelet.
	bare := renderSpec(ng, node, clusterInputs{})
	require.Empty(t, bare.ContainerRuntime.RegistryConfigOwner,
		"a cluster that never heard of an agent goes on as it did before")

	// The intent on its own. Reachable whenever the object has not arrived yet, and
	// an end state when it never does — a manifest the contest refused, or one the
	// per-group cap left out.
	intentOnly := renderSpec(ng, node, clusterInputs{RegistryAgentMode: true})
	require.Empty(t, intentOnly.ContainerRuntime.RegistryConfigOwner,
		"nodelet keeps writing registry.d until the node actually has the agent")

	// The pod on its own. Some other module may name a static pod whatever it likes;
	// who owns registry.d still follows from the registry module.
	podOnly := renderSpec(ng, node, clusterInputs{NodeStaticPodRequests: withAgentStaticPod()})
	require.Empty(t, podOnly.ContainerRuntime.RegistryConfigOwner)

	claimed := renderSpec(ng, node, clusterInputs{
		RegistryAgentMode:     true,
		NodeStaticPodRequests: withAgentStaticPod(),
	})
	require.Equal(t, internalv1alpha1.RegistryConfigOwnerRegistryAgent, claimed.ContainerRuntime.RegistryConfigOwner)

	// And it is taken back the moment the registry module stops saying so: the
	// directory goes back to nodelet without anybody editing a node.
	require.Empty(t, renderSpec(ng, node, clusterInputs{}).ContainerRuntime.RegistryConfigOwner)
}

// The API server defaults an unnamed type to Immutable; an Engine render that left
// it out would never equal the object it wrote, and every pass would patch it.
func TestTheEngineRenderNamesItsSystemType(t *testing.T) {
	ng := immutableGroup("worker")

	spec := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{})

	require.Equal(t, internalv1alpha1.SystemTypeImmutable, spec.SystemType)
}

// The spec's sections are omitzero; an Engine render from no inputs still sends the
// ones the cluster always has something to say in, and leaves out a section that
// would carry nothing but defaults, which the node holds.
func TestTheEngineRenderSendsWhatItDecides(t *testing.T) {
	raw, err := json.Marshal(renderSpec(immutableGroup("worker"), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{}))
	require.NoError(t, err)

	for _, section := range []string{"kernel", "network", "storage", "updatePolicy", "kubelet"} {
		require.Contains(t, string(raw), `"`+section+`":{`)
	}
	require.NotContains(t, string(raw), `"containerRuntime"`)
	require.NotContains(t, string(raw), `"windows"`)
}

// The warnings name limits of an Engine node. bashible applies maxPods 1500 and
// RollingUpdate as written, so the same words on its group would be false.
func TestRecordClampedSettingsLeavesABashibleGroupAlone(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{}
	r.Recorder = recorder

	for _, systemType := range []v1.SystemType{v1.SystemTypeMutable, ""} {
		ng := mutableGroup("worker", systemType)
		ng.Spec.Kubelet = &v1.KubeletSpec{MaxPods: ptr.To(int32(1500))}
		ng.Spec.Disruptions = &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeRollingUpdate}
		r.recordClampedSettings(ng)
	}
	close(recorder.Events)

	var messages []string
	for event := range recorder.Events {
		messages = append(messages, event)
	}
	require.Empty(t, messages)
}

// The node holds every window of its group, as bashible's nodes do, and no clamp
// warning is left to explain a lost one. Days are a set on the node: a group that
// repeats a day must not get its whole document refused for it.
func TestRenderUpdatePolicyCarriesEveryWindow(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			SystemType: v1.SystemTypeImmutable,
			Disruptions: &v1.DisruptionsSpec{
				ApprovalMode: v1.DisruptionApprovalModeManual,
				Automatic: &v1.AutomaticDisruptionSpec{Windows: []v1.DisruptionWindow{
					{From: "22:00", To: "23:00", Days: []string{"Sat", "Sat", "Sun"}},
					{From: "1:00", To: "3:00"},
				}},
			},
		},
	}

	require.Equal(t, internalv1alpha1.UpdatePolicy{
		Mode: string(v1.DisruptionApprovalModeManual),
		Windows: []internalv1alpha1.UpdateWindow{
			{From: "22:00", To: "23:00", Days: []string{"Sat", "Sun"}},
			{From: "1:00", To: "3:00"},
		},
	}, renderUpdatePolicy(ng))

	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{}
	r.Recorder = recorder
	r.recordClampedSettings(ng)
	close(recorder.Events)
	require.Empty(t, recorder.Events, "nothing was clamped")
}
