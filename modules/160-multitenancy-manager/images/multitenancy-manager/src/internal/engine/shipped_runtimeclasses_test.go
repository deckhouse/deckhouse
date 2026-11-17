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
	"k8s.io/apimachinery/pkg/runtime"

	"controller/api/v1alpha1"
	"controller/internal/testutil"
)

// TestShippedRuntimeClassReferences checks that the shipped runtimeclasses references find
// spec.runtimeClassName in every kind that carries it: a pod and the pod template of each built-in
// workload. Pods of the built-in workloads are written by kube-controller-manager, which the grant
// webhooks skip, so a kind whose path misses the name would let a project use any RuntimeClass.
func TestShippedRuntimeClassReferences(t *testing.T) {
	var refs []*v1alpha1.GrantableClusterResourceReference
	for _, ref := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceReference](t, "GrantableClusterResourceReference") {
		if ref.Spec.GrantableClusterResourceName == "runtimeclasses" {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		t.Fatal("no runtimeclasses references in the rendered templates")
	}

	const name = "gvisor"
	rc := name
	podSpec := corev1.PodSpec{RuntimeClassName: &rc, Containers: []corev1.Container{{Name: "app", Image: "app"}}}
	template := corev1.PodTemplateSpec{Spec: podSpec}

	cases := []struct {
		group, resource string
		obj             runtime.Object
	}{
		{"", "pods", &corev1.Pod{Spec: podSpec}},
		{"", "replicationcontrollers", &corev1.ReplicationController{Spec: corev1.ReplicationControllerSpec{Template: &template}}},
		{"", "podtemplates", &corev1.PodTemplate{Template: template}},
		{"apps", "deployments", &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: template}}},
		{"apps", "statefulsets", &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: template}}},
		{"apps", "daemonsets", &appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: template}}},
		{"apps", "replicasets", &appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{Template: template}}},
		{"batch", "jobs", &batchv1.Job{Spec: batchv1.JobSpec{Template: template}}},
		{"batch", "cronjobs", &batchv1.CronJob{Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: template}}}}},
	}
	for _, c := range cases {
		t.Run(c.resource, func(t *testing.T) {
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.obj)
			if err != nil {
				t.Fatal(err)
			}
			var matched []string
			for _, ref := range refs {
				if !RuleMatches(ref.Spec.Rule, c.group, "v1", c.resource) {
					continue
				}
				fp, ok := SelectFieldPath(ref.Spec.FieldPaths, c.group, "v1", c.resource)
				if !ok {
					t.Fatalf("%s matches %s/%s but has no field path for it", ref.Name, c.group, c.resource)
				}
				if fp.Defaulting != v1alpha1.DefaultingNone {
					t.Fatalf("%s defaults %s/%s with %q; RuntimeClass has no cluster default", ref.Name, c.group, c.resource, fp.Defaulting)
				}
				names, err := StringValuesAt(factory(), obj, fp.Path)
				if err != nil {
					t.Fatal(err)
				}
				if len(names) != 1 || names[0] != name {
					t.Fatalf("%s reads %v at %s, want [%s]", ref.Name, names, fp.Path, name)
				}
				matched = append(matched, ref.Name)
			}
			if len(matched) != 1 {
				t.Fatalf("%s/%s is matched by %v, want exactly one runtimeclasses reference", c.group, c.resource, matched)
			}
		})
	}
}
