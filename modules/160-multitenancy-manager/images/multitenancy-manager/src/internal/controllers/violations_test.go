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

package controllers

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/naming"
)

// violationsMapper knows StorageClass (the granted resource) and PersistentVolumeClaim (the usage
// object the reference points at).
func violationsMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "storage.k8s.io", Version: "v1"}, {Group: "", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, meta.RESTScopeNamespace)
	return m
}

func pvc(ns, name, storageClass string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass},
	}
}

func storageClassGrant(name, allowed string) *v1alpha1.ClusterResourceGrantPolicy {
	return &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{allowed}}},
		},
	}
}

// violationsFixture is the phantom-alert scenario of the audit: two allow-list policies on the same
// resource in the same project, a PVC that uses the class the second policy allows, and a PVC that
// uses a class no policy allows.
func violationsFixture(t *testing.T) (*ProjectReconciler, client.Client) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a", "env": "prod"}}}
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
		},
	}
	ref := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses-pvc"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "storageclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.storageClassName"}},
		},
	}
	objs := []client.Object{
		ns, def, ref,
		storageClassGrant("grant-a", "fast"),
		storageClassGrant("grant-b", "slow"),
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}, Provisioner: "x"},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "slow"}, Provisioner: "x"},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Provisioner: "x"},
		pvc("team-a", "uses-slow", "slow"),
		pvc("team-a", "uses-other", "other"),
	}
	cl := buildClient(t, objs...)
	r := &ProjectReconciler{Client: cl, Mapper: violationsMapper(), Usage: cl, Factory: jsonpath.NewWithCache()}
	return r, cl
}

func reconcileNamespace(t *testing.T, r *ProjectReconciler, ns string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: ns}}); err != nil {
		t.Fatalf("reconcile %s: %v", ns, err)
	}
}

// violationSeries reads the published series without creating one: GaugeVec.With would register an
// empty series for the label set it is asked about, and the count assertions would see it.
func violationSeries(project, object string) float64 {
	ch := make(chan prometheus.Metric, 64)
	go func() {
		grantViolations.Collect(ch)
		close(ch)
	}()
	total := 0.0
	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			continue
		}
		labels := map[string]string{}
		for _, l := range pb.GetLabel() {
			labels[l.GetName()] = l.GetValue()
		}
		if labels["project"] == project && labels["violating_object_name"] == object && labels["grant"] == "grant-a" {
			total += pb.GetGauge().GetValue()
		}
	}
	return total
}

// TestViolations_UnionOfPoliciesIsNotAViolation: the webhook admits a PVC whose class any applicable
// policy allows, so the metric must not report it; the class no policy allows is reported once per
// applicable policy, exactly as the hook used to.
func TestViolations_UnionOfPoliciesIsNotAViolation(t *testing.T) {
	grantViolations.Reset()
	r, _ := violationsFixture(t)
	reconcileNamespace(t, r, "team-a")

	if got := violationSeries("team-a", "uses-slow"); got != 0 {
		t.Fatalf("a PVC allowed by the second policy was reported as a violation (phantom alert): %v", got)
	}
	if got := violationSeries("team-a", "uses-other"); got != 1 {
		t.Fatalf("a PVC no policy allows must be reported, got %v", got)
	}
	// One series per applicable policy naming the definition: grant-a and grant-b.
	if got := testutil.CollectAndCount(grantViolations); got != 2 {
		t.Fatalf("expected 2 series (one per applicable policy), got %d", got)
	}
}

// TestViolations_SeriesExpireWithTheObject: deleting the violating object removes its series on the
// next reconcile instead of leaving a phantom firing alert behind.
func TestViolations_SeriesExpireWithTheObject(t *testing.T) {
	grantViolations.Reset()
	r, cl := violationsFixture(t)
	reconcileNamespace(t, r, "team-a")
	if got := violationSeries("team-a", "uses-other"); got != 1 {
		t.Fatalf("precondition: violation expected, got %v", got)
	}

	if err := cl.Delete(context.Background(), pvc("team-a", "uses-other", "other")); err != nil {
		t.Fatalf("delete pvc: %v", err)
	}
	reconcileNamespace(t, r, "team-a")
	if got := testutil.CollectAndCount(grantViolations); got != 0 {
		t.Fatalf("series must disappear with the object, %d left", got)
	}
}

// TestViolations_PolicyChangeClearsTheSeries: widening a policy so the class becomes allowed clears
// the violation without touching the object.
func TestViolations_PolicyChangeClearsTheSeries(t *testing.T) {
	grantViolations.Reset()
	r, cl := violationsFixture(t)
	reconcileNamespace(t, r, "team-a")

	g := &v1alpha1.ClusterResourceGrantPolicy{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "grant-a"}, g); err != nil {
		t.Fatal(err)
	}
	g.Spec.Resources[0].Allowed = append(g.Spec.Resources[0].Allowed, "other")
	if err := cl.Update(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	reconcileNamespace(t, r, "team-a")
	if got := testutil.CollectAndCount(grantViolations); got != 0 {
		t.Fatalf("violation must clear once a policy allows the class, %d series left", got)
	}
}

// TestViolations_NamespaceGoneClearsTheSeries: a namespace that disappears takes its series with it.
func TestViolations_NamespaceGoneClearsTheSeries(t *testing.T) {
	grantViolations.Reset()
	r, cl := violationsFixture(t)
	reconcileNamespace(t, r, "team-a")

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	if err := cl.Delete(context.Background(), ns); err != nil {
		t.Fatal(err)
	}
	reconcileNamespace(t, r, "team-a")
	if got := testutil.CollectAndCount(grantViolations); got != 0 {
		t.Fatalf("series of a deleted namespace must be dropped, %d left", got)
	}
}

// TestViolations_WildcardGroupIsSkipped mirrors the hook: a rule with apiGroups ["*"] cannot be
// enumerated and is skipped rather than failing the reconcile.
func TestViolations_WildcardGroupIsSkipped(t *testing.T) {
	grantViolations.Reset()
	r, cl := violationsFixture(t)
	ref := &v1alpha1.GrantableClusterResourceReference{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "storageclasses-pvc"}, ref); err != nil {
		t.Fatal(err)
	}
	ref.Spec.Rule.APIGroups = []string{"*"}
	if err := cl.Update(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	reconcileNamespace(t, r, "team-a")
	if got := testutil.CollectAndCount(grantViolations); got != 0 {
		t.Fatalf("a wildcard group must be skipped, got %d series", got)
	}
}
