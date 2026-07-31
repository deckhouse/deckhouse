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

package nodetemplate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func reconcileSingle(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile single %s failed: %v", name, err)
	}
}

// --- SetupWatches ---

type fakeWatcher struct {
	watchedObjects []client.Object
}

func (w *fakeWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *fakeWatcher) Watches(object client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
	w.watchedObjects = append(w.watchedObjects, object)
}
func (w *fakeWatcher) WatchesRawSource(_ source.Source)      {}
func (w *fakeWatcher) WithEventFilter(_ predicate.Predicate) {}

func TestSetupWatches_WatchesNodeGroup(t *testing.T) {
	r := &Reconciler{}
	w := &fakeWatcher{}
	r.SetupWatches(w)

	if len(w.watchedObjects) != 1 {
		t.Fatalf("expected exactly one watch, got %d", len(w.watchedObjects))
	}
	if _, ok := w.watchedObjects[0].(*v1.NodeGroup); !ok {
		t.Fatalf("expected NodeGroup to be watched, got %T", w.watchedObjects[0])
	}
}

// --- reconcileSingleNode (single-node reconcile path) ---

func TestReconcileSingle_AppliesTemplate(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:     v1.NodeTypeStatic,
			NodeTemplate: &v1.NodeTemplate{Labels: map[string]string{"template-label": "yes"}},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{nodeGroupNameLabel: "worker"},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileSingle(t, r, "worker-1")

	updated := getNode(t, r, "worker-1")
	if updated.Labels["template-label"] != "yes" {
		t.Fatalf("expected template label applied via single-node path")
	}
	if updated.Labels["node.deckhouse.io/type"] != string(v1.NodeTypeStatic) {
		t.Fatalf("expected node type label, got %q", updated.Labels["node.deckhouse.io/type"])
	}
}

func TestReconcileSingle_NodeNotFound_NoError(t *testing.T) {
	r := testReconciler(t)
	reconcileSingle(t, r, "ghost")
}

func TestReconcileSingle_NoNodeGroupLabel_Skips(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "orphan"}}
	r := testReconciler(t, node)
	reconcileSingle(t, r, "orphan")

	updated := getNode(t, r, "orphan")
	if _, ok := updated.Labels["node.deckhouse.io/type"]; ok {
		t.Fatalf("expected no template applied to label-less node")
	}
}

func TestReconcileSingle_NodeGroupNotFound_Skips(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{nodeGroupNameLabel: "missing-ng"},
		},
	}
	r := testReconciler(t, node)
	reconcileSingle(t, r, "worker-1")

	updated := getNode(t, r, "worker-1")
	if _, ok := updated.Labels["node.deckhouse.io/type"]; ok {
		t.Fatalf("expected no template applied when NodeGroup is missing")
	}
}

func TestReconcileSingle_NoChange_NoError(t *testing.T) {
	// CloudEphemeral non-CAPI node with no uninitialized taint and matching taints: nothing to do.
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{nodeGroupNameLabel: "worker"},
		},
	}
	r := testReconciler(t, ng, node)
	reconcileSingle(t, r, "worker-1")
}

func TestReconcileAll_SkipsNodesWithoutLabelAndMissingNG(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	unmanaged := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "unmanaged"}}
	orphanNG := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "orphan",
			Labels: map[string]string{nodeGroupNameLabel: "ghost-ng"},
		},
	}
	managed := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{nodeGroupNameLabel: "worker"},
		},
	}

	r := testReconciler(t, ng, unmanaged, orphanNG, managed)
	reconcileAll(t, r)

	// Unmanaged node must be flagged in the metric.
	if v := gaugeValue(t, unmanagedNodesGauge, "node", "unmanaged"); v != 1 {
		t.Fatalf("expected unmanaged node metric = 1, got %v", v)
	}
	// Orphan node (NG missing) must be left untouched.
	if _, ok := getNode(t, r, "orphan").Labels["node.deckhouse.io/type"]; ok {
		t.Fatalf("expected orphan node untouched")
	}
	// Managed node must have the template applied.
	if getNode(t, r, "worker-1").Labels["node.deckhouse.io/type"] != string(v1.NodeTypeStatic) {
		t.Fatalf("expected managed node to receive template")
	}
}

// --- master node handling ---

func TestReconcileAll_Master_AddsRoleLabelsAndFixesTaints(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudPermanent},
	}
	// master role taint present in node spec but absent from NG template -> removed.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "master-0",
			Labels: map[string]string{nodeGroupNameLabel: "master"},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: masterNodeRoleKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileAll(t, r)

	updated := getNode(t, r, "master-0")
	if _, ok := updated.Labels[controlPlaneTaintKey]; !ok {
		t.Fatalf("expected control-plane label on master node")
	}
	if _, ok := updated.Labels[masterNodeRoleKey]; !ok {
		t.Fatalf("expected master role label on master node")
	}
	if taintSliceHasKey(updated.Spec.Taints, masterNodeRoleKey) {
		t.Fatalf("expected legacy master role taint to be removed, got %+v", updated.Spec.Taints)
	}
}

func TestReconcileAll_Master_KeepsRoleTaintWhenInTemplate(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudPermanent,
			NodeTemplate: &v1.NodeTemplate{
				Taints: []corev1.Taint{{Key: masterNodeRoleKey, Effect: corev1.TaintEffectNoSchedule}},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "master-0",
			Labels: map[string]string{nodeGroupNameLabel: "master"},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: masterNodeRoleKey, Effect: corev1.TaintEffectNoSchedule}},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileAll(t, r)

	updated := getNode(t, r, "master-0")
	if !taintSliceHasKey(updated.Spec.Taints, masterNodeRoleKey) {
		t.Fatalf("expected master role taint kept when present in template, got %+v", updated.Spec.Taints)
	}
}

// --- fixMasterTaints (direct, all branches) ---

func TestFixMasterTaints(t *testing.T) {
	tests := []struct {
		name       string
		nodeTaints []corev1.Taint
		ngTaints   []corev1.Taint
		wantHasKey bool // whether masterNodeRoleKey remains
		wantLen    int
	}{
		{
			name:       "empty node taints returns unchanged",
			nodeTaints: nil,
			ngTaints:   nil,
			wantHasKey: false,
			wantLen:    0,
		},
		{
			name:       "control-plane present keeps everything",
			nodeTaints: []corev1.Taint{{Key: controlPlaneTaintKey}, {Key: masterNodeRoleKey}},
			ngTaints:   nil,
			wantHasKey: true,
			wantLen:    2,
		},
		{
			name:       "legacy master taint removed when not in NG and no control-plane",
			nodeTaints: []corev1.Taint{{Key: masterNodeRoleKey}, {Key: "dedicated"}},
			ngTaints:   nil,
			wantHasKey: false,
			wantLen:    1,
		},
		{
			name:       "master taint kept when present in NG template",
			nodeTaints: []corev1.Taint{{Key: masterNodeRoleKey}},
			ngTaints:   []corev1.Taint{{Key: masterNodeRoleKey}},
			wantHasKey: true,
			wantLen:    1,
		},
		{
			name:       "no master taint at all returns unchanged",
			nodeTaints: []corev1.Taint{{Key: "dedicated"}},
			ngTaints:   nil,
			wantHasKey: false,
			wantLen:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixMasterTaints(tt.nodeTaints, tt.ngTaints)
			if taintSliceHasKey(got, masterNodeRoleKey) != tt.wantHasKey {
				t.Fatalf("hasKey(master) = %v, want %v (got %+v)", !tt.wantHasKey, tt.wantHasKey, got)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (got %+v)", len(got), tt.wantLen, got)
			}
		})
	}
}

// --- syncMissingMasterTaintMetric ---

func TestSyncMissingMasterTaintMetric(t *testing.T) {
	masterNG := func(taints []corev1.Taint) v1.NodeGroup {
		ng := v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "master"}}
		if taints != nil {
			ng.Spec.NodeTemplate = &v1.NodeTemplate{Taints: taints}
		}
		return ng
	}
	workerNG := v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	oneNode := []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "n0"}}}
	twoNodes := []corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "n0"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "n1"}},
	}

	tests := []struct {
		name       string
		nodeGroups []v1.NodeGroup
		nodes      []corev1.Node
		wantMetric float64
	}{
		{
			name:       "no master nodegroup, metric stays zero",
			nodeGroups: []v1.NodeGroup{workerNG},
			nodes:      twoNodes,
			wantMetric: 0,
		},
		{
			name:       "single-node topology skips metric",
			nodeGroups: []v1.NodeGroup{masterNG(nil)},
			nodes:      oneNode,
			wantMetric: 0,
		},
		{
			name:       "missing control-plane taint sets metric",
			nodeGroups: []v1.NodeGroup{masterNG(nil), workerNG},
			nodes:      twoNodes,
			wantMetric: 1,
		},
		{
			name:       "control-plane taint present clears metric",
			nodeGroups: []v1.NodeGroup{masterNG([]corev1.Taint{{Key: controlPlaneTaintKey}}), workerNG},
			nodes:      twoNodes,
			wantMetric: 0,
		},
	}

	r := &Reconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.syncMissingMasterTaintMetric(tt.nodeGroups, tt.nodes)
			if v := gaugeValue(t, missingMasterTaintGauge, "name", "master"); v != tt.wantMetric {
				t.Fatalf("metric = %v, want %v", v, tt.wantMetric)
			}
		})
	}
}

// --- helpers: getTemplateLabels / getTemplateAnnotations nil branches ---

func TestGetTemplateLabelsAndAnnotations(t *testing.T) {
	noTemplate := &v1.NodeGroup{}
	if got := getTemplateLabels(noTemplate); len(got) != 0 {
		t.Fatalf("expected empty labels for nil template, got %+v", got)
	}
	if got := getTemplateAnnotations(noTemplate); len(got) != 0 {
		t.Fatalf("expected empty annotations for nil template, got %+v", got)
	}

	emptyTemplate := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{}}}
	if got := getTemplateLabels(emptyTemplate); len(got) != 0 {
		t.Fatalf("expected empty labels for nil map, got %+v", got)
	}
	if got := getTemplateAnnotations(emptyTemplate); len(got) != 0 {
		t.Fatalf("expected empty annotations for nil map, got %+v", got)
	}

	withData := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{
		Labels:      map[string]string{"a": "1"},
		Annotations: map[string]string{"b": "2"},
	}}}
	if got := getTemplateLabels(withData); got["a"] != "1" {
		t.Fatalf("expected cloned labels, got %+v", got)
	}
	if got := getTemplateAnnotations(withData); got["b"] != "2" {
		t.Fatalf("expected cloned annotations, got %+v", got)
	}
}

// --- helpers: nodeChanged branches ---

func TestNodeChanged(t *testing.T) {
	base := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"a": "1"},
			Annotations: map[string]string{"b": "2"},
		},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{{Key: "k"}}},
	}

	tests := []struct {
		name string
		mut  func(n *corev1.Node)
		want bool
	}{
		{name: "identical", mut: func(_ *corev1.Node) {}, want: false},
		{name: "labels differ", mut: func(n *corev1.Node) { n.Labels = map[string]string{"a": "9"} }, want: true},
		{name: "annotations differ", mut: func(n *corev1.Node) { n.Annotations = map[string]string{"b": "9"} }, want: true},
		{name: "taints differ", mut: func(n *corev1.Node) { n.Spec.Taints = []corev1.Taint{{Key: "other"}} }, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modified := base.DeepCopy()
			tt.mut(modified)
			if got := nodeChanged(base, modified); got != tt.want {
				t.Fatalf("nodeChanged = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- fixCloudNodeTaints: divergent-taints early return branch ---

func TestFixCloudNodeTaints_TemplateAddsTaint_NoStripUninitialized(t *testing.T) {
	// When merging the NG template introduces a new taint, mergeTaints result
	// differs from the current node taints, so the function returns early and
	// the uninitialized taint is NOT removed.
	node := &corev1.Node{
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{
		Taints: []corev1.Taint{{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}},
	}}}

	fixCloudNodeTaints(node, ng)

	if !taintSliceHasKey(node.Spec.Taints, nodeUninitializedTaintKey) {
		t.Fatalf("expected uninitialized taint kept when template diverges, got %+v", node.Spec.Taints)
	}
}

func TestFixCloudNodeTaints_TemplateChangesTaintValue_NoStrip(t *testing.T) {
	// Same key+effect but a different value: mergeTaints overwrites the value,
	// so taintSliceEqual returns false (value mismatch) and the function returns early.
	node := &corev1.Node{
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "old", Effect: corev1.TaintEffectNoSchedule},
				{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}
	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{
		Taints: []corev1.Taint{{Key: "dedicated", Value: "new", Effect: corev1.TaintEffectNoSchedule}},
	}}}

	fixCloudNodeTaints(node, ng)

	if !taintSliceHasKey(node.Spec.Taints, nodeUninitializedTaintKey) {
		t.Fatalf("expected uninitialized taint kept when template changes a taint value, got %+v", node.Spec.Taints)
	}
}

func TestFixCloudNodeTaints_OnlyUninitialized_Cleared(t *testing.T) {
	node := &corev1.Node{
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{}}

	fixCloudNodeTaints(node, ng)

	if node.Spec.Taints != nil {
		t.Fatalf("expected taints nil after removing only uninitialized taint, got %+v", node.Spec.Taints)
	}
}

// --- taintSliceEqual ---

func TestTaintSliceEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []corev1.Taint
		b    []corev1.Taint
		want bool
	}{
		{
			name: "equal",
			a:    []corev1.Taint{{Key: "k", Value: "v", Effect: corev1.TaintEffectNoSchedule}},
			b:    []corev1.Taint{{Key: "k", Value: "v", Effect: corev1.TaintEffectNoSchedule}},
			want: true,
		},
		{
			name: "different length",
			a:    []corev1.Taint{{Key: "k"}},
			b:    nil,
			want: false,
		},
		{
			name: "same length different key",
			a:    []corev1.Taint{{Key: "a", Effect: corev1.TaintEffectNoSchedule}},
			b:    []corev1.Taint{{Key: "b", Effect: corev1.TaintEffectNoSchedule}},
			want: false,
		},
		{
			name: "same key+effect different value",
			a:    []corev1.Taint{{Key: "k", Value: "x", Effect: corev1.TaintEffectNoSchedule}},
			b:    []corev1.Taint{{Key: "k", Value: "y", Effect: corev1.TaintEffectNoSchedule}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taintSliceEqual(tt.a, tt.b); got != tt.want {
				t.Fatalf("taintSliceEqual = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- applyTemplateTaints: removal + nil/nil branches ---

func TestApplyTemplateTaints(t *testing.T) {
	t.Run("nil template and lastApplied returns empty changed", func(t *testing.T) {
		got, changed := applyTemplateTaints(nil, nil, nil)
		if !changed || len(got) != 0 {
			t.Fatalf("expected empty+changed, got %+v changed=%v", got, changed)
		}
	})

	t.Run("removes taint no longer in template", func(t *testing.T) {
		actual := []corev1.Taint{{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}}
		lastApplied := []corev1.Taint{{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}}
		got, changed := applyTemplateTaints(actual, nil, lastApplied)
		if !changed {
			t.Fatalf("expected changed=true when taint removed")
		}
		if taintSliceHasKey(got, "dedicated") {
			t.Fatalf("expected dedicated taint removed, got %+v", got)
		}
	})

	t.Run("updates taint value", func(t *testing.T) {
		actual := []corev1.Taint{{Key: "dedicated", Value: "old", Effect: corev1.TaintEffectNoSchedule}}
		template := []corev1.Taint{{Key: "dedicated", Value: "new", Effect: corev1.TaintEffectNoSchedule}}
		got, changed := applyTemplateTaints(actual, template, nil)
		if !changed {
			t.Fatalf("expected changed when taint value updated")
		}
		if len(got) != 1 || got[0].Value != "new" {
			t.Fatalf("expected updated taint value, got %+v", got)
		}
	})

	t.Run("no change when actual matches template", func(t *testing.T) {
		taint := corev1.Taint{Key: "dedicated", Value: "v", Effect: corev1.TaintEffectNoSchedule}
		got, changed := applyTemplateTaints([]corev1.Taint{taint}, []corev1.Taint{taint}, []corev1.Taint{taint})
		if changed {
			t.Fatalf("expected no change, got %+v", got)
		}
	})
}

// --- applyNodeTemplate: consumes existing last-applied to strip excess keys ---

func TestApplyNodeTemplate_StripsExcessFromLastApplied(t *testing.T) {
	lastApplied := map[string]interface{}{
		"labels":      map[string]string{"old-label": "x"},
		"annotations": map[string]string{"old-annot": "y"},
		"taints":      []corev1.Taint{{Key: "old-taint", Effect: corev1.TaintEffectNoSchedule}},
	}
	raw, err := json.Marshal(lastApplied)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				"old-label":           "x",
				metalLBmemberLabelKey: "keep-untouched-source",
			},
			Annotations: map[string]string{
				"old-annot":                       "y",
				lastAppliedNodeTemplateAnnotation: string(raw),
				heartbeatAnnotationKey:            "skip",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: "old-taint", Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeStatic,
			NodeTemplate: &v1.NodeTemplate{
				Labels: map[string]string{"new-label": "z"},
			},
		},
	}

	if err := applyNodeTemplate(node, ng); err != nil {
		t.Fatalf("applyNodeTemplate: %v", err)
	}

	if _, ok := node.Labels["old-label"]; ok {
		t.Fatalf("expected excess label removed, got %+v", node.Labels)
	}
	if node.Labels["new-label"] != "z" {
		t.Fatalf("expected new label applied")
	}
	if _, ok := node.Annotations["old-annot"]; ok {
		t.Fatalf("expected excess annotation removed, got %+v", node.Annotations)
	}
	if taintSliceHasKey(node.Spec.Taints, "old-taint") {
		t.Fatalf("expected excess taint removed, got %+v", node.Spec.Taints)
	}
}

// gaugeValue reads a single label-pair value from a gauge vec.
func gaugeValue(t *testing.T, g *prometheus.GaugeVec, label, value string) float64 {
	t.Helper()
	m := &dto.Metric{}
	gauge, err := g.GetMetricWith(prometheus.Labels{label: value})
	if err != nil {
		t.Fatalf("get metric: %v", err)
	}
	if err := gauge.Write(m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetGauge().GetValue()
}
