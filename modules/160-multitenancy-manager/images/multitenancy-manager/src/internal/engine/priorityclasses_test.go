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

package engine

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/testutil"
)

// TestShippedPriorityClassCatalogFields projects the shipped priorityclasses catalogFields onto a
// typical PriorityClass.
func TestShippedPriorityClassCatalogFields(t *testing.T) {
	def := shippedDefinition(t, "priorityclasses")
	never := corev1.PreemptNever
	pc := &schedulingv1.PriorityClass{
		TypeMeta:         metav1.TypeMeta{APIVersion: "scheduling.k8s.io/v1", Kind: "PriorityClass"},
		ObjectMeta:       metav1.ObjectMeta{Name: "develop"},
		Value:            1000,
		GlobalDefault:    true,
		Description:      "For workloads in development environments.",
		PreemptionPolicy: &never,
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pc)
	if err != nil {
		t.Fatal(err)
	}
	assertProjected(t, projected(def.Spec.CatalogFields, obj), map[string]string{
		"value":            `1000`,
		"preemptionPolicy": `"Never"`,
		"globalDefault":    `true`,
		"description":      `"For workloads in development environments."`,
	})
}

// shippedReference returns the named GrantableClusterResourceReference of the module's own Helm chart.
func shippedReference(t *testing.T, name string) *v1alpha1.GrantableClusterResourceReference {
	t.Helper()
	for _, ref := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceReference](t, "GrantableClusterResourceReference") {
		if ref.Name == name {
			return ref
		}
	}
	t.Fatalf("reference %s is not in the rendered template", name)
	return nil
}

// TestShippedPriorityClassReferences checks, for every kind the shipped priorityclasses references
// cover, that the field path selected for the kind reads priorityClassName from a real object of it,
// and the defaulting mode: None for pods, where the built-in Priority admission plugin has already
// set the class and the priority computed from it, and for PodTemplate, which nothing turns into pods;
// FillEmpty for the pod templates of the workloads.
func TestShippedPriorityClassReferences(t *testing.T) {
	const class = "production-medium"
	podSpec := corev1.PodSpec{PriorityClassName: class, Containers: []corev1.Container{{Name: "app", Image: "app"}}}
	template := corev1.PodTemplateSpec{Spec: podSpec}

	cases := []struct {
		reference, group, resource string
		obj                        runtime.Object
		defaulting                 v1alpha1.DefaultingMode
	}{
		{"priorityclasses-pod", "", "pods", &corev1.Pod{Spec: podSpec}, v1alpha1.DefaultingNone},
		{"priorityclasses-pod", "", "replicationcontrollers", &corev1.ReplicationController{Spec: corev1.ReplicationControllerSpec{Template: &template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-pod", "", "podtemplates", &corev1.PodTemplate{Template: template}, v1alpha1.DefaultingNone},
		{"priorityclasses-apps", "apps", "deployments", &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-apps", "apps", "statefulsets", &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-apps", "apps", "daemonsets", &appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-apps", "apps", "replicasets", &appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{Template: template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-batch", "batch", "jobs", &batchv1.Job{Spec: batchv1.JobSpec{Template: template}}, v1alpha1.DefaultingFillEmpty},
		{"priorityclasses-batch", "batch", "cronjobs", &batchv1.CronJob{Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: template}}}}, v1alpha1.DefaultingFillEmpty},
	}

	factory := jsonpath.NewWithCache()
	for _, c := range cases {
		t.Run(c.resource, func(t *testing.T) {
			ref := shippedReference(t, c.reference)
			fp, ok := SelectFieldPath(ref.Spec.FieldPaths, c.group, "v1", c.resource)
			if !ok {
				t.Fatalf("no field path of %s selected for %s", c.reference, c.resource)
			}
			if fp.Defaulting != c.defaulting {
				t.Fatalf("defaulting: got %q want %q", fp.Defaulting, c.defaulting)
			}
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.obj)
			if err != nil {
				t.Fatal(err)
			}
			names, err := StringValuesAt(factory, obj, fp.Path)
			if err != nil {
				t.Fatalf("path %s: %v", fp.Path, err)
			}
			if len(names) != 1 || names[0] != class {
				t.Fatalf("path %s: got %v want [%s]", fp.Path, names, class)
			}
		})
	}
}
