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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhouse v1 scheme: %v", err)
	}
	return scheme
}

func testReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := testScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &Reconciler{Base: register.Base{Client: cl}}
}

func reconcileAll(t *testing.T, r *Reconciler) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: allRequestName}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
}

func getNode(t *testing.T, r *Reconciler, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node
}

func TestReconcile_StaticNode_AppliesTemplate(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeStatic,
			NodeTemplate: &v1.NodeTemplate{
				Labels:      map[string]string{"template-label": "yes"},
				Annotations: map[string]string{"template-annotation": "yes"},
				Taints: []corev1.Taint{
					{Key: "dedicated", Value: "workload", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				nodeGroupNameLabel: "worker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileAll(t, r)
	updated := getNode(t, r, "worker-1")

	if updated.Labels["template-label"] != "yes" {
		t.Fatalf("expected template label to be applied")
	}
	if _, ok := updated.Labels["node-role.kubernetes.io/worker"]; !ok {
		t.Fatalf("expected node role label from nodegroup")
	}
	if updated.Labels["node.deckhouse.io/type"] != string(v1.NodeTypeStatic) {
		t.Fatalf("expected node type label to be Static, got %q", updated.Labels["node.deckhouse.io/type"])
	}
	if updated.Annotations["template-annotation"] != "yes" {
		t.Fatalf("expected template annotation to be applied")
	}
	if _, ok := updated.Annotations[lastAppliedNodeTemplateAnnotation]; !ok {
		t.Fatalf("expected last-applied annotation")
	}
	if updated.Annotations["cluster-autoscaler.kubernetes.io/scale-down-disabled"] != "true" {
		t.Fatalf("expected scale-down-disabled annotation to be true")
	}
	if len(updated.Spec.Taints) != 1 || updated.Spec.Taints[0].Key != "dedicated" {
		t.Fatalf("expected uninitialized taint removed and dedicated taint kept, got %+v", updated.Spec.Taints)
	}
}

func TestReconcile_CloudEphemeral_NonCAPI_OnlyFixesTaints(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			NodeTemplate: &v1.NodeTemplate{
				Labels: map[string]string{"template-label": "yes"},
				Taints: []corev1.Taint{
					{Key: "dedicated", Value: "monitoring", Effect: corev1.TaintEffectNoSchedule},
				},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				nodeGroupNameLabel: "worker",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "monitoring", Effect: corev1.TaintEffectNoSchedule},
				{Key: nodeUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileAll(t, r)
	updated := getNode(t, r, "worker-1")

	if _, ok := updated.Labels["template-label"]; ok {
		t.Fatalf("did not expect full template apply for non-CAPI cloud ephemeral node")
	}
	if len(updated.Spec.Taints) != 1 || updated.Spec.Taints[0].Key != "dedicated" {
		t.Fatalf("expected only dedicated taint left, got %+v", updated.Spec.Taints)
	}
}

func TestReconcile_CloudEphemeral_CAPI_AppliesTemplate(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			NodeTemplate: &v1.NodeTemplate{
				Labels:      map[string]string{"template-label": "yes"},
				Annotations: map[string]string{"template-annotation": "yes"},
			},
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				nodeGroupNameLabel: "worker",
			},
			Annotations: map[string]string{
				clusterAPIAnnotationKey: "machine-1",
			},
		},
	}

	r := testReconciler(t, ng, node)
	reconcileAll(t, r)
	updated := getNode(t, r, "worker-1")

	if updated.Labels["template-label"] != "yes" {
		t.Fatalf("expected template label for CAPI node")
	}
	if updated.Annotations["template-annotation"] != "yes" {
		t.Fatalf("expected template annotation for CAPI node")
	}
	if _, ok := updated.Annotations[lastAppliedNodeTemplateAnnotation]; !ok {
		t.Fatalf("expected last-applied annotation for CAPI node")
	}
}
