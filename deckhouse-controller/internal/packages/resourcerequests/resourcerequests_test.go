// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcerequests

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// parse turns a manifest into the unstructured shape Apply works on.
func parse(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	object := make(map[string]any)
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &object))

	return &unstructured.Unstructured{Object: object}
}

// containerResources digs the resources out of a container of the pod spec at path.
func containerResources(t *testing.T, resource *unstructured.Unstructured, name string, path ...string) map[string]any {
	t.Helper()

	containers, found, err := unstructured.NestedSlice(resource.Object, append(path, "containers")...)
	require.NoError(t, err)
	require.True(t, found)

	for _, entry := range containers {
		spec, ok := entry.(map[string]any)
		require.True(t, ok)

		if spec["name"] == name {
			resources, _ := spec["resources"].(map[string]any)

			return resources
		}
	}

	t.Fatalf("container %q not found", name)

	return nil
}

func int32Ptr(v int32) *int32 { return &v }

const deploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller
spec:
  replicas: 1
  template:
    spec:
      initContainers:
        - name: migrate
          image: migrate:v1
      containers:
        - name: controller
          image: controller:v1
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
        - name: kube-rbac-proxy
          image: proxy:v1
`

func TestApplyOverlaysReplicasAndResources(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	unmatched, err := Apply([]*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(3),
		Containers: []Container{{
			Name:     "controller",
			Requests: map[string]string{"memory": "128Mi"},
			Limits:   map[string]string{"cpu": "1", "memory": "512Mi"},
		}},
	}})
	require.NoError(t, err)
	require.Empty(t, unmatched)

	replicas, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(3), replicas)

	resources := containerResources(t, deployment, "controller", "spec", "template", "spec")

	// The chart's cpu request survives: the overlay merges key by key rather than
	// replacing the whole list.
	require.Equal(t, map[string]any{"cpu": "10m", "memory": "128Mi"}, resources["requests"])
	require.Equal(t, map[string]any{"cpu": "1", "memory": "512Mi"}, resources["limits"])
}

func TestApplyLeavesUnnamedContainersAlone(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	_, err := Apply([]*unstructured.Unstructured{deployment}, []Request{{
		Kind: "Deployment",
		Name: "controller",
		Containers: []Container{
			{Name: "controller", Limits: map[string]string{"cpu": "1"}},
			// A container the workload does not have is ignored, not an error.
			{Name: "absent", Limits: map[string]string{"cpu": "2"}},
		},
	}})
	require.NoError(t, err)

	require.Nil(t, containerResources(t, deployment, "kube-rbac-proxy", "spec", "template", "spec"))
}

func TestApplyResizesInitContainers(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	_, err := Apply([]*unstructured.Unstructured{deployment}, []Request{{
		Kind:       "Deployment",
		Name:       "controller",
		Containers: []Container{{Name: "migrate", Limits: map[string]string{"memory": "64Mi"}}},
	}})
	require.NoError(t, err)

	initContainers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	require.NoError(t, err)
	require.True(t, found)

	spec, ok := initContainers[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"limits": map[string]any{"memory": "64Mi"}}, spec["resources"])
}

func TestApplyReachesCronJobPodSpec(t *testing.T) {
	cronJob := parse(t, `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
              image: backup:v1
`)

	_, err := Apply([]*unstructured.Unstructured{cronJob}, []Request{{
		Kind:       "CronJob",
		Name:       "backup",
		Containers: []Container{{Name: "backup", Limits: map[string]string{"memory": "1Gi"}}},
	}})
	require.NoError(t, err)

	resources := containerResources(t, cronJob, "backup", "spec", "jobTemplate", "spec", "template", "spec")
	require.Equal(t, map[string]any{"memory": "1Gi"}, resources["limits"])
}

func TestApplyReachesBarePodSpec(t *testing.T) {
	pod := parse(t, `
apiVersion: v1
kind: Pod
metadata:
  name: probe
spec:
  containers:
    - name: probe
      image: probe:v1
`)

	_, err := Apply([]*unstructured.Unstructured{pod}, []Request{{
		Kind:       "Pod",
		Name:       "probe",
		Containers: []Container{{Name: "probe", Requests: map[string]string{"cpu": "5m"}}},
	}})
	require.NoError(t, err)

	resources := containerResources(t, pod, "probe", "spec")
	require.Equal(t, map[string]any{"cpu": "5m"}, resources["requests"])
}

func TestApplyReportsUnmatchedRequests(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	unmatched, err := Apply([]*unstructured.Unstructured{deployment}, []Request{
		{Kind: "Deployment", Name: "controller"},
		{Kind: "StatefulSet", Name: "database", Replicas: int32Ptr(2)},
	})
	require.NoError(t, err)
	require.Len(t, unmatched, 1)
	require.Equal(t, "database", unmatched[0].Name)
}

// The CR schema rejects replicas on a kind that has none; the overlay refuses it
// too, so a request that reached it another way cannot write a field the
// workload does not have.
func TestApplyRejectsReplicasOnKindWithoutThem(t *testing.T) {
	daemonSet := parse(t, `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agent
spec:
  template:
    spec:
      containers:
        - name: agent
`)

	_, err := Apply([]*unstructured.Unstructured{daemonSet}, []Request{{
		Kind:     "DaemonSet",
		Name:     "agent",
		Replicas: int32Ptr(2),
	}})
	require.ErrorContains(t, err, "replicas")
}

func TestApplyRejectsUnsupportedKind(t *testing.T) {
	service := parse(t, `
apiVersion: v1
kind: Service
metadata:
  name: controller
`)

	_, err := Apply([]*unstructured.Unstructured{service}, []Request{{Kind: "Service", Name: "controller"}})
	require.ErrorContains(t, err, "unsupported workload kind")
}

func TestApplyIsANoOpWithoutRequests(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	before := deployment.DeepCopy()

	unmatched, err := Apply([]*unstructured.Unstructured{deployment}, nil)
	require.NoError(t, err)
	require.Empty(t, unmatched)
	require.Equal(t, before, deployment)
}

func TestEqual(t *testing.T) {
	requests := []Request{
		{Kind: "Deployment", Name: "a", Replicas: int32Ptr(1)},
		{Kind: "Deployment", Name: "b"},
	}

	require.True(t, Equal(nil, nil))
	require.True(t, Equal(requests, []Request{
		{Kind: "Deployment", Name: "a", Replicas: int32Ptr(1)},
		{Kind: "Deployment", Name: "b"},
	}))

	// Compared by value, not by pointer identity.
	require.False(t, Equal(requests, []Request{
		{Kind: "Deployment", Name: "a", Replicas: int32Ptr(2)},
		{Kind: "Deployment", Name: "b"},
	}))

	require.False(t, Equal(requests, requests[:1]))
	require.False(t, Equal(nil, []Request{{Kind: "Deployment", Name: "a"}}))
}
