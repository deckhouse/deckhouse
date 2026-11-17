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
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/jsonpath"
	"controller/internal/naming"
)

func controlledBy(apiVersion, kind, name string) []metav1.OwnerReference {
	return []metav1.OwnerReference{{APIVersion: apiVersion, Kind: kind, Name: name, UID: types.UID("uid-" + name), Controller: ptr.To(true)}}
}

// TestViolations_CopiesOfACoveredOwnerAreNotReported: a Deployment with a RuntimeClass the project may
// not use is reported once. Its ReplicaSet and Pod are copies of its template, owned by resources the
// same definition covers, so they are not reported again. A Pod created directly is reported, and so
// is a PersistentVolumeClaim of a StatefulSet for storageclasses, which does not cover StatefulSets.
func TestViolations_CopiesOfACoveredOwnerAreNotReported(t *testing.T) {
	grantViolations.Reset()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a", "env": "prod"}}}
	def := func(name, group, kind string) *v1alpha1.GrantableClusterResourceDefinition {
		return &v1alpha1.GrantableClusterResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
				GrantedResource:     &v1alpha1.GrantedResource{APIGroup: group, Kind: kind},
				DefaultAvailability: v1alpha1.AvailabilityAll,
			},
		}
	}
	ref := func(name, definition, group string, resources []string, fieldPaths ...v1alpha1.FieldPath) *v1alpha1.GrantableClusterResourceReference {
		return &v1alpha1.GrantableClusterResourceReference{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
				GrantableClusterResourceName: definition,
				Rule:                         v1alpha1.UsageRule{APIGroups: []string{group}, APIVersions: []string{"v1"}, Resources: resources},
				FieldPaths:                   fieldPaths,
			},
		}
	}
	grant := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "grant-a"},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{
				{ResourceName: "runtimeclasses", Allowed: []string{"runc"}},
				{ResourceName: "storageclasses", Allowed: []string{"fast"}},
			},
		},
	}
	kata := "kata"
	podSpec := corev1.PodSpec{RuntimeClassName: &kata, Containers: []corev1.Container{{Name: "app", Image: "app"}}}
	template := corev1.PodTemplateSpec{Spec: podSpec}
	other := "other"

	objs := []client.Object{
		ns, grant,
		def("runtimeclasses", "node.k8s.io", "RuntimeClass"),
		ref("runtimeclasses-core", "runtimeclasses", "", []string{"pods"}, v1alpha1.FieldPath{Path: "$.spec.runtimeClassName"}),
		ref("runtimeclasses-apps", "runtimeclasses", "apps", []string{"deployments", "replicasets"}, v1alpha1.FieldPath{Path: "$.spec.template.spec.runtimeClassName"}),
		def("storageclasses", "storage.k8s.io", "StorageClass"),
		ref("storageclasses-pvc", "storageclasses", "", []string{"persistentvolumeclaims"}, v1alpha1.FieldPath{Path: "$.spec.storageClassName"}),
		&nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "runc"}, Handler: "runc"},
		&nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata"}, Handler: "kata"},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}, Provisioner: "x"},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Provisioner: "x"},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team-a"}, Spec: appsv1.DeploymentSpec{Template: template}},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "team-a", OwnerReferences: controlledBy("apps/v1", "Deployment", "web")},
			Spec:       appsv1.ReplicaSetSpec{Template: template},
		},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1-x", Namespace: "team-a", OwnerReferences: controlledBy("apps/v1", "ReplicaSet", "web-1")}, Spec: podSpec},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bare", Namespace: "team-a"}, Spec: podSpec},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data-db-0", Namespace: "team-a", OwnerReferences: controlledBy("apps/v1", "StatefulSet", "db")},
			Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &other},
		},
	}

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, nodev1.AddToScheme, storagev1.AddToScheme, v1alpha1.AddToScheme, grantsv1alpha2.AddToScheme, v1alpha3.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.AvailableClusterResource{}, &v1alpha1.GrantableClusterResourceDefinition{},
			&v1alpha1.GrantableClusterResourceReference{}, &grantsv1alpha2.ClusterResourceGrantPolicy{}).
		Build()

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}, {Group: "apps", Version: "v1"}, {Group: "node.k8s.io", Version: "v1"}, {Group: "storage.k8s.io", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "node.k8s.io", Version: "v1", Kind: "RuntimeClass"}, meta.RESTScopeRoot)
	mapper.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	for _, gvk := range []schema.GroupVersionKind{
		{Version: "v1", Kind: "Pod"}, {Version: "v1", Kind: "PersistentVolumeClaim"},
		{Group: "apps", Version: "v1", Kind: "Deployment"}, {Group: "apps", Version: "v1", Kind: "ReplicaSet"}, {Group: "apps", Version: "v1", Kind: "StatefulSet"},
	} {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}

	r := &ProjectReconciler{Client: cl, Mapper: mapper, Usage: cl, Factory: jsonpath.NewWithCache()}
	reconcileNamespace(t, r, "team-a")

	for object, want := range map[string]float64{"web": 1, "bare": 1, "data-db-0": 1, "web-1": 0, "web-1-x": 0} {
		if got := violationSeries("team-a", object); got != want {
			t.Errorf("%s: got %v series, want %v", object, got, want)
		}
	}
	if got := testutil.CollectAndCount(grantViolations); got != 3 {
		t.Fatalf("expected 3 series, got %d", got)
	}
}
