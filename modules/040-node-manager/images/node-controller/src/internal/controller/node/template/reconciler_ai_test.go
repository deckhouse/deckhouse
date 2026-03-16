//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package template

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	return s
}

func newTestReconciler(cl client.Client) *Reconciler {
	r := &Reconciler{}
	r.Client = cl
	return r
}

func reconcileNode(t *testing.T, r *Reconciler, nodeName string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: nodeName},
	})
	require.NoError(t, err)
	return result
}

func getNode(t *testing.T, cl client.Client, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: name}, node)
	require.NoError(t, err)
	return node
}

func getLastApplied(t *testing.T, node *corev1.Node) *lastAppliedNodeTemplate {
	t.Helper()
	raw, ok := node.Annotations[lastAppliedNodeTemplateAnnotation]
	require.True(t, ok, "last-applied-node-template annotation must exist")
	require.NotEmpty(t, raw)
	var la lastAppliedNodeTemplate
	require.NoError(t, json.Unmarshal([]byte(raw), &la))
	return &la
}

func hasTaintWithKey(taints []corev1.Taint, key string) bool {
	for _, t := range taints {
		if t.Key == key {
			return true
		}
	}
	return false
}

// TestAI_NodeWithoutGroupLabel verifies that a Node without the
// node.deckhouse.io/group label is skipped by the reconciler.
func TestAI_NodeWithoutGroupLabel(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "unmanaged-node",
			Labels: map[string]string{},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(node).Build()
	r := newTestReconciler(cl)

	result := reconcileNode(t, r, "unmanaged-node")
	assert.Equal(t, ctrl.Result{}, result)

	// Node should be unchanged — no annotations added.
	got := getNode(t, cl, "unmanaged-node")
	assert.Empty(t, got.Annotations, "unmanaged node should not get any annotations")
}

// TestAI_NodeGroupNotFound verifies that reconciliation is skipped when the
// NodeGroup referenced by the node does not exist.
func TestAI_NodeGroupNotFound(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan-node",
			Labels: map[string]string{
				"node.deckhouse.io/group": "nonexistent-ng",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(node).Build()
	r := newTestReconciler(cl)

	result := reconcileNode(t, r, "orphan-node")
	assert.Equal(t, ctrl.Result{}, result)

	// Node should be unchanged.
	got := getNode(t, cl, "orphan-node")
	_, hasLastApplied := got.Annotations[lastAppliedNodeTemplateAnnotation]
	assert.False(t, hasLastApplied, "node should not have last-applied annotation when NG not found")
}

// TestAI_EmptyNodeTemplate_CloudEphemeral verifies that when a CloudEphemeral
// NodeGroup has no nodeTemplate, the uninitialized taint is removed and the
// node-role label is added.
func TestAI_EmptyNodeTemplate_CloudEphemeral(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group":         "wor-ker",
				"node-role.kubernetes.io/wor-ker": "",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// Uninitialized taint must be removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
		"uninitialized taint should be removed")

	// Node-role label must be present.
	assert.Equal(t, "", got.Labels["node-role.kubernetes.io/wor-ker"])

	// last-applied-node-template annotation must be set.
	la := getLastApplied(t, got)
	assert.Empty(t, la.Labels)
	assert.Empty(t, la.Annotations)
	assert.Empty(t, la.Taints)
}

// TestAI_EmptyNodeTemplate_Static verifies that a Static NodeGroup without
// nodeTemplate gets last-applied annotation, scale-down-disabled annotation,
// node type label and node-role label.
func TestAI_EmptyNodeTemplate_Static(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// last-applied-node-template annotation with empty values.
	la := getLastApplied(t, got)
	assert.Empty(t, la.Labels)
	assert.Empty(t, la.Annotations)
	assert.Empty(t, la.Taints)

	// scale-down-disabled annotation for Static type.
	assert.Equal(t, "true", got.Annotations[scaleDownDisabledAnnotation])

	// node type label.
	assert.Equal(t, "Static", got.Labels[nodeTypeLabel])

	// node-role label.
	assert.Equal(t, "", got.Labels["node-role.kubernetes.io/wor-ker"])
}

// TestAI_NodeTemplateWithLabelsAnnotationsTaints verifies that labels,
// annotations and taints from nodeTemplate are applied to the node, and
// the uninitialized taint is removed.
func TestAI_NodeTemplateWithLabelsAnnotationsTaints(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Annotations: map[string]string{"new": "new"},
				Labels: map[string]string{
					"new":                     "new",
					"node.deckhouse.io/group": "wor-ker",
				},
				Taints: []corev1.Taint{
					{Key: "new", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// Labels from template.
	assert.Equal(t, "new", got.Labels["new"])
	assert.Equal(t, "wor-ker", got.Labels["node.deckhouse.io/group"])

	// Annotations from template.
	assert.Equal(t, "new", got.Annotations["new"])

	// Taints from template, uninitialized removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"))
	assert.True(t, hasTaintWithKey(got.Spec.Taints, "new"))

	// last-applied-node-template.
	la := getLastApplied(t, got)
	assert.Equal(t, "new", la.Labels["new"])
	assert.Equal(t, "wor-ker", la.Labels["node.deckhouse.io/group"])
	assert.Equal(t, "new", la.Annotations["new"])
	require.Len(t, la.Taints, 1)
	assert.Equal(t, "new", la.Taints[0].Key)
}

// TestAI_RemoveOldLabelsAnnotationsTaints verifies that keys previously
// applied (in last-applied) but removed from nodeTemplate are cleaned up.
func TestAI_RemoveOldLabelsAnnotationsTaints(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Annotations: map[string]string{
				lastAppliedNodeTemplateAnnotation: `{"labels":{"old-old":"old"},"annotations":{"old-old":"old"},"taints":[{"key":"old-old","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// Old labels/annotations/taints must be removed.
	_, hasOldLabel := got.Labels["old-old"]
	assert.False(t, hasOldLabel, "old label should be removed")

	_, hasOldAnnotation := got.Annotations["old-old"]
	assert.False(t, hasOldAnnotation, "old annotation should be removed")

	assert.False(t, hasTaintWithKey(got.Spec.Taints, "old-old"), "old taint should be removed")

	// Uninitialized taint also removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"))

	// last-applied must be empty.
	la := getLastApplied(t, got)
	assert.Empty(t, la.Labels)
	assert.Empty(t, la.Annotations)
	assert.Empty(t, la.Taints)
}

// TestAI_UpdateAddNewKeysToExisting verifies that new keys from nodeTemplate
// are added while existing ones are preserved.
func TestAI_UpdateAddNewKeysToExisting(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Annotations: map[string]string{"a": "a", "new": "new"},
				Labels:      map[string]string{"a": "a", "new": "new"},
				Taints: []corev1.Taint{
					{Key: "a", Effect: corev1.TaintEffectNoSchedule},
					{Key: "new", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Annotations: map[string]string{
				lastAppliedNodeTemplateAnnotation: `{"labels":{"a":"a"},"annotations":{"a":"a"},"taints":[{"key":"a","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	la := getLastApplied(t, got)
	assert.Equal(t, "a", la.Annotations["a"])
	assert.Equal(t, "new", la.Annotations["new"])
	assert.Equal(t, "a", la.Labels["a"])
	assert.Equal(t, "new", la.Labels["new"])
	require.Len(t, la.Taints, 2)

	taintKeys := map[string]bool{}
	for _, taint := range la.Taints {
		taintKeys[taint.Key] = true
	}
	assert.True(t, taintKeys["a"])
	assert.True(t, taintKeys["new"])
}

// TestAI_UpdateRemoveOldKeysKeepCurrent verifies that old keys removed from
// nodeTemplate are deleted from the node while current ones remain.
func TestAI_UpdateRemoveOldKeysKeepCurrent(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Annotations: map[string]string{"a": "a"},
				Labels:      map[string]string{"a": "a"},
				Taints: []corev1.Taint{
					{Key: "a", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Annotations: map[string]string{
				lastAppliedNodeTemplateAnnotation: `{"labels":{"a":"a","old":"old"},"annotations":{"a":"a","old":"old"},"taints":[{"key":"a","effect":"NoSchedule"},{"key":"old","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// Verify updated last-applied.
	la := getLastApplied(t, got)
	assert.Equal(t, map[string]string{"a": "a"}, la.Annotations)
	assert.Equal(t, map[string]string{"a": "a"}, la.Labels)
	require.Len(t, la.Taints, 1)
	assert.Equal(t, "a", la.Taints[0].Key)

	// Verify node labels/annotations.
	assert.Equal(t, "a", got.Labels["a"])
	_, hasOldLabel := got.Labels["old"]
	assert.False(t, hasOldLabel, "old label should be removed")

	assert.Equal(t, "a", got.Annotations["a"])
	_, hasOldAnnotation := got.Annotations["old"]
	assert.False(t, hasOldAnnotation, "old annotation should be removed")

	// Verify taints.
	assert.True(t, hasTaintWithKey(got.Spec.Taints, "a"))
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "old"), "old taint should be removed")
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
		"uninitialized taint should be removed")

	// Verify scale-down-disabled for Static.
	assert.Equal(t, "true", got.Annotations[scaleDownDisabledAnnotation])

	// Verify node type label.
	assert.Equal(t, "Static", got.Labels[nodeTypeLabel])

	// Verify node-role label.
	assert.Equal(t, "", got.Labels["node-role.kubernetes.io/wor-ker"])
}

// TestAI_NodeRoleDeckhouseLabels verifies that node-role.deckhouse.io/* labels
// from the nodeTemplate are properly applied to nodes.
func TestAI_NodeRoleDeckhouseLabels(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node.deckhouse.io/group":         "wor-ker",
					"node-role.deckhouse.io/system":   "",
					"node-role.deckhouse.io/stateful": "",
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	la := getLastApplied(t, got)
	assert.Equal(t, "", la.Labels["node-role.deckhouse.io/system"])
	assert.Equal(t, "", la.Labels["node-role.deckhouse.io/stateful"])
	assert.Equal(t, "wor-ker", la.Labels["node.deckhouse.io/group"])
	assert.Empty(t, la.Annotations)
	assert.Empty(t, la.Taints)

	// Labels on node.
	assert.Equal(t, "", got.Labels["node-role.deckhouse.io/system"])
	assert.Equal(t, "", got.Labels["node-role.deckhouse.io/stateful"])
}

// TestAI_NGWithoutTaintsNodeWithTaints verifies that when the NodeGroup has no
// taints in nodeTemplate but the node has pre-existing taints, those taints
// are removed (since last-applied is empty).
func TestAI_NGWithoutTaintsNodeWithTaints(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node.deckhouse.io/group":         "wor-ker",
					"node-role.deckhouse.io/system":   "",
					"node-role.deckhouse.io/stateful": "",
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// When template has nil taints and last-applied is nil, applyTemplateTaints
	// returns an empty slice — all existing taints are effectively replaced.
	assert.Empty(t, got.Spec.Taints, "taints should be empty")
}

// TestAI_AddAnnotationToExistingNode verifies that adding an annotation via
// nodeTemplate works on a node that already has system labels.
func TestAI_AddAnnotationToExistingNode(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Annotations: map[string]string{"test": "test"},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/type":          "Static",
				"node.deckhouse.io/group":         "wor-ker",
				"node-role.kubernetes.io/wor-ker": "",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")
	assert.Equal(t, "test", got.Annotations["test"])
}

// TestAI_SetEmptyNodeTemplate_RemovePreviouslyApplied verifies that setting
// an empty nodeTemplate removes labels, annotations and taints that were
// previously applied via last-applied-node-template.
func TestAI_SetEmptyNodeTemplate_RemovePreviouslyApplied(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:     deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Annotations: map[string]string{
				"a":                               "a",
				lastAppliedNodeTemplateAnnotation: `{"annotations":{"a":"a"},"labels":{"a":"a"},"taints":[{"key":"a","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"a":                               "a",
				"node.deckhouse.io/group":         "wor-ker",
				"node-role.kubernetes.io/wor-ker": "",
				"node.deckhouse.io/type":          "Static",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// last-applied must be empty.
	la := getLastApplied(t, got)
	assert.Empty(t, la.Labels)
	assert.Empty(t, la.Annotations)
	assert.Empty(t, la.Taints)

	// Label "a" should be removed.
	_, hasA := got.Labels["a"]
	assert.False(t, hasA, "label a should be removed")

	// Annotation "a" should be removed.
	_, hasAAnnotation := got.Annotations["a"]
	assert.False(t, hasAAnnotation, "annotation a should be removed")

	// Taint "a" should be removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "a"), "taint a should be removed")

	// Uninitialized taint should be removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
		"uninitialized taint should be removed")
}

// TestAI_MasterNodeGroup_ControlPlaneAndMasterLabels verifies that the master
// NodeGroup automatically gets control-plane and master node-role labels.
func TestAI_MasterNodeGroup_ControlPlaneAndMasterLabels(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudPermanent,
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-master-0",
			Labels: map[string]string{
				"node.deckhouse.io/group": "master",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node-role.deckhouse.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "kube-master-0")

	got := getNode(t, cl, "kube-master-0")

	// Control-plane and master labels must be set.
	val, ok := got.Labels["node-role.kubernetes.io/master"]
	assert.True(t, ok, "master role label should exist")
	assert.Equal(t, "", val)

	val, ok = got.Labels["node-role.kubernetes.io/control-plane"]
	assert.True(t, ok, "control-plane role label should exist")
	assert.Equal(t, "", val)
}

// TestAI_SingleNodeCluster_ControlPlaneTaintRemoved verifies that in a single
// node cluster scenario where the control-plane taint was previously applied
// but then removed from NG template, both control-plane and master taints
// are cleaned up.
func TestAI_SingleNodeCluster_ControlPlaneTaintRemoved(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudPermanent,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-master-0",
			Annotations: map[string]string{
				lastAppliedNodeTemplateAnnotation: `{"annotations":{},"labels":{"node-role.kubernetes.io/control-plane":"","node-role.kubernetes.io/master":""},"taints":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"kubernetes.io/hostname":                "kube-master-0",
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
				"node.deckhouse.io/group":               "master",
				"node.deckhouse.io/type":                "CloudPermanent",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "kube-master-0")

	got := getNode(t, cl, "kube-master-0")

	// Node should have no taints — control-plane taint was in last-applied
	// but not in current template, so it gets removed.
	assert.Empty(t, got.Spec.Taints, "node should have no taints after removing control-plane taint from template")
}

// TestAI_SingleNodeCluster_BothTaintsRemoved verifies that when a master node
// has both control-plane and master taints but the NG template has no taints,
// both are removed.
func TestAI_SingleNodeCluster_BothTaintsRemoved(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudPermanent,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-master-0",
			Annotations: map[string]string{
				lastAppliedNodeTemplateAnnotation: `{"annotations":{},"labels":{"node-role.kubernetes.io/control-plane":"","node-role.kubernetes.io/master":""},"taints":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]}`,
			},
			Labels: map[string]string{
				"kubernetes.io/hostname":                "kube-master-0",
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
				"node.deckhouse.io/group":               "master",
				"node.deckhouse.io/type":                "CloudPermanent",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "kube-master-0")

	got := getNode(t, cl, "kube-master-0")

	// Both taints should be removed.
	assert.Empty(t, got.Spec.Taints, "node should have no taints")
}

// TestAI_MasterNG_ExplicitMasterTaintPreserved verifies that when the master
// NG template explicitly includes the master taint, it is preserved on the node
// even though control-plane taint is not present.
func TestAI_MasterNG_ExplicitMasterTaintPreserved(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudPermanent,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
				},
				Taints: []corev1.Taint{
					{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-master-0",
			Labels: map[string]string{
				"kubernetes.io/hostname":                "kube-master-0",
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
				"node.deckhouse.io/group":               "master",
				"node.deckhouse.io/type":                "CloudPermanent",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "kube-master-0")

	got := getNode(t, cl, "kube-master-0")

	// Master taint should be preserved since it is explicitly in the NG template.
	require.Len(t, got.Spec.Taints, 1)
	assert.Equal(t, "node-role.kubernetes.io/master", got.Spec.Taints[0].Key)
	assert.Equal(t, corev1.TaintEffectNoSchedule, got.Spec.Taints[0].Effect)
}

// TestAI_CloudEphemeral_UninitializedTaintRemoved_TaintsWithDifferentEffects
// verifies that the uninitialized taint is removed while user-defined taints
// (even with the same key but different effects) are preserved.
// Note: The controller uses taint.Key as map key so two taints with same key
// but different effects will collapse to one. This test reflects controller behavior.
func TestAI_CloudEphemeral_UninitializedTaintRemoved_TaintsWithDifferentEffects(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Taints: []corev1.Taint{
					{Key: "node-role", Effect: corev1.TaintEffectNoExecute, Value: "monitoring"},
					{Key: "node-role", Effect: corev1.TaintEffectNoSchedule, Value: "monitoring"},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group":         "wor-ker",
				"node-role.kubernetes.io/wor-ker": "",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node-role", Effect: corev1.TaintEffectNoExecute, Value: "monitoring"},
				{Key: "node-role", Effect: corev1.TaintEffectNoSchedule, Value: "monitoring"},
				{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")

	// Uninitialized taint should be removed.
	assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
		"uninitialized taint should be removed")

	// node-role taint should still exist (controller collapses by key, so one remains).
	assert.True(t, hasTaintWithKey(got.Spec.Taints, "node-role"),
		"node-role taint should be preserved")
}

// TestAI_BashibleUninitializedTaintPreserved verifies the behavior when both
// uninitialized and bashible-uninitialized taints exist on a node.
// When the NG has no nodeTemplate and no last-applied annotation, the controller
// replaces all taints with an empty set (applyTemplateTaints returns empty when
// both template and lastApplied are nil). This removes ALL taints including
// bashible-uninitialized. The regular uninitialized taint is also removed via
// taintsWithoutKey. This test documents the current controller behavior.
//
// With a nodeTemplate that includes bashible-uninitialized as a desired taint,
// the taint IS preserved.
func TestAI_BashibleUninitializedTaintPreserved(t *testing.T) {
	t.Run("with_nodeTemplate_preserving_bashible_taint", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				NodeTemplate: &deckhousev1.NodeTemplate{
					Taints: []corev1.Taint{
						{Key: "node.deckhouse.io/bashible-uninitialized", Effect: corev1.TaintEffectNoSchedule},
					},
				},
			},
		}
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "wor-ker",
				Labels: map[string]string{
					"node.deckhouse.io/group":         "wor-ker",
					"node-role.kubernetes.io/wor-ker": "",
				},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
					{Key: "node.deckhouse.io/bashible-uninitialized", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		}

		cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
		r := newTestReconciler(cl)

		reconcileNode(t, r, "wor-ker")

		got := getNode(t, cl, "wor-ker")

		// Regular uninitialized taint should be removed.
		assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
			"regular uninitialized taint should be removed")

		// Bashible-uninitialized taint should be preserved because it is in the template.
		assert.True(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/bashible-uninitialized"),
			"bashible-uninitialized taint should be preserved")
	})

	t.Run("without_nodeTemplate_all_taints_cleared", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
			},
		}
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "wor-ker",
				Labels: map[string]string{
					"node.deckhouse.io/group":         "wor-ker",
					"node-role.kubernetes.io/wor-ker": "",
				},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{Key: "node.deckhouse.io/uninitialized", Effect: corev1.TaintEffectNoSchedule},
					{Key: "node.deckhouse.io/bashible-uninitialized", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		}

		cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
		r := newTestReconciler(cl)

		reconcileNode(t, r, "wor-ker")

		got := getNode(t, cl, "wor-ker")

		// When both template and lastApplied are nil, applyTemplateTaints returns
		// an empty slice, clearing all taints. Then uninitialized is also removed
		// explicitly. Both taints end up removed.
		assert.False(t, hasTaintWithKey(got.Spec.Taints, "node.deckhouse.io/uninitialized"),
			"regular uninitialized taint should be removed")
		assert.Empty(t, got.Spec.Taints,
			"all taints cleared when template and lastApplied are both nil")
	})
}

// TestAI_LastAppliedAnnotationUpdated verifies that the last-applied-node-template
// annotation is correctly updated on every reconciliation.
func TestAI_LastAppliedAnnotationUpdated(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels:      map[string]string{"l1": "v1"},
				Annotations: map[string]string{"a1": "v1"},
				Taints: []corev1.Taint{
					{Key: "t1", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	reconcileNode(t, r, "wor-ker")

	got := getNode(t, cl, "wor-ker")
	la := getLastApplied(t, got)

	assert.Equal(t, map[string]string{"l1": "v1"}, la.Labels)
	assert.Equal(t, map[string]string{"a1": "v1"}, la.Annotations)
	require.Len(t, la.Taints, 1)
	assert.Equal(t, "t1", la.Taints[0].Key)
	assert.Equal(t, corev1.TaintEffectNoSchedule, la.Taints[0].Effect)
}

// TestAI_ScaleDownDisabledForStaticTypes verifies that the
// cluster-autoscaler.kubernetes.io/scale-down-disabled annotation is set for
// Static, CloudPermanent, and CloudStatic node types, but not for CloudEphemeral.
func TestAI_ScaleDownDisabledForStaticTypes(t *testing.T) {
	tests := []struct {
		nodeType    deckhousev1.NodeType
		expectAnnot bool
	}{
		{deckhousev1.NodeTypeStatic, true},
		{deckhousev1.NodeTypeCloudPermanent, true},
		{deckhousev1.NodeTypeCloudStatic, true},
		{deckhousev1.NodeTypeCloudEphemeral, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.nodeType), func(t *testing.T) {
			ng := &deckhousev1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ng"},
				Spec: deckhousev1.NodeGroupSpec{
					NodeType: tt.nodeType,
				},
			}
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"node.deckhouse.io/group": "test-ng",
					},
				},
			}

			cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
			r := newTestReconciler(cl)

			reconcileNode(t, r, "test-node")

			got := getNode(t, cl, "test-node")
			if tt.expectAnnot {
				assert.Equal(t, "true", got.Annotations[scaleDownDisabledAnnotation],
					"scale-down-disabled should be set for %s", tt.nodeType)
			} else {
				val, exists := got.Annotations[scaleDownDisabledAnnotation]
				if exists {
					assert.NotEqual(t, "true", val,
						"scale-down-disabled should not be true for %s", tt.nodeType)
				}
			}
		})
	}
}

// TestAI_NodeNotFound verifies that reconciling a non-existent node does not
// return an error.
func TestAI_NodeNotFound(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	r := newTestReconciler(cl)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_IdempotentReconciliation verifies that running reconciliation twice
// produces the same result (idempotency).
func TestAI_IdempotentReconciliation(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "wor-ker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels:      map[string]string{"key": "value"},
				Annotations: map[string]string{"ann": "val"},
				Taints: []corev1.Taint{
					{Key: "taint-key", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wor-ker",
			Labels: map[string]string{
				"node.deckhouse.io/group": "wor-ker",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(ng, node).Build()
	r := newTestReconciler(cl)

	// First reconciliation.
	reconcileNode(t, r, "wor-ker")
	first := getNode(t, cl, "wor-ker")

	// Second reconciliation — should be a no-op.
	reconcileNode(t, r, "wor-ker")
	second := getNode(t, cl, "wor-ker")

	assert.Equal(t, first.Labels, second.Labels)
	assert.Equal(t, first.Annotations, second.Annotations)
	assert.Equal(t, first.Spec.Taints, second.Spec.Taints)
}
