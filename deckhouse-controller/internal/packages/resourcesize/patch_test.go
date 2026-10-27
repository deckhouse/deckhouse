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

package resourcesize

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/werf/nelm/v2/pkg/resource/spec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const releaseNamespace = "d8-test"

// apply runs the requests' patches over resources the way nelm does on render,
// replacing each resource's content with the patched one, and returns the
// requests that matched none of them.
func apply(t *testing.T, resources []*unstructured.Unstructured, requests []Request) error {
	t.Helper()

	patches, err := Patches(requests)
	if err != nil {
		return err
	}

	compiled, err := spec.CompilePatches(patches)
	require.NoError(t, err)

	for _, resource := range resources {
		meta := spec.NewResourceMetaFromUnstructured(resource, releaseNamespace, "chart/templates/workload.yaml")

		patched, err := spec.ApplyPatches(t.Context(), compiled, meta, releaseNamespace, resource)
		require.NoError(t, err)

		resource.Object = patched.Object
	}

	return nil
}

// parse turns a manifest into the unstructured shape nelm patches.
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

func TestPatchesOverlaysReplicasAndResources(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
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

func TestPatchesLeavesUnnamedContainersAlone(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
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

func TestPatchesResizesInitContainers(t *testing.T) {
	deployment := parse(t, deploymentManifest)

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
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

func TestPatchesReachesCronJobPodSpec(t *testing.T) {
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

	err := apply(t, []*unstructured.Unstructured{cronJob}, []Request{{
		Kind:       "CronJob",
		Name:       "backup",
		Containers: []Container{{Name: "backup", Limits: map[string]string{"memory": "1Gi"}}},
	}})
	require.NoError(t, err)

	resources := containerResources(t, cronJob, "backup", "spec", "jobTemplate", "spec", "template", "spec")
	require.Equal(t, map[string]any{"memory": "1Gi"}, resources["limits"])
}

func TestPatchesReachesBarePodSpec(t *testing.T) {
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

	err := apply(t, []*unstructured.Unstructured{pod}, []Request{{
		Kind:       "Pod",
		Name:       "probe",
		Containers: []Container{{Name: "probe", Requests: map[string]string{"cpu": "5m"}}},
	}})
	require.NoError(t, err)

	resources := containerResources(t, pod, "probe", "spec")
	require.Equal(t, map[string]any{"cpu": "5m"}, resources["requests"])
}

// The CR schema rejects replicas on a kind that has none; the overlay refuses it
// too, so a request that reached it another way cannot write a field the
// workload does not have.
func TestPatchesRejectsReplicasOnKindWithoutThem(t *testing.T) {
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

	err := apply(t, []*unstructured.Unstructured{daemonSet}, []Request{{
		Kind:     "DaemonSet",
		Name:     "agent",
		Replicas: int32Ptr(2),
	}})
	require.ErrorContains(t, err, "replicas")
}

func TestPatchesRejectsUnsupportedKind(t *testing.T) {
	service := parse(t, `
apiVersion: v1
kind: Service
metadata:
  name: controller
`)

	err := apply(t, []*unstructured.Unstructured{service}, []Request{{Kind: "Service", Name: "controller"}})
	require.ErrorContains(t, err, "unsupported workload kind")
}

func TestPatchesIsANoOpWithoutRequests(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	before := deployment.DeepCopy()

	err := apply(t, []*unstructured.Unstructured{deployment}, nil)
	require.NoError(t, err)
	require.Equal(t, before, deployment)
}

// The patch matches its workload by kind and name alone, and writes nothing the
// request did not ask for: no init containers list, no empty resources.
func TestPatchesTouchOnlyWhatTheyAddress(t *testing.T) {
	target := parse(t, deploymentManifest)
	other := parse(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook
spec:
  template:
    spec:
      containers:
        - name: controller
          image: webhook:v1
`)
	before := other.DeepCopy()

	err := apply(t, []*unstructured.Unstructured{target, other}, []Request{{
		Kind: "Deployment",
		Name: "controller",
		Containers: []Container{
			{Name: "controller", Limits: map[string]string{"cpu": "1"}},
			{Name: "kube-rbac-proxy", Requests: map[string]string{}},
		},
	}})
	require.NoError(t, err)

	require.Equal(t, before, other)
	require.Nil(t, containerResources(t, target, "kube-rbac-proxy", "spec", "template", "spec"))
}

// nelm reads a name wrapped in slashes as a regexp; no object can be named that
// way, so such a request must patch nothing rather than every Deployment.
func TestPatchesSkipNamesThatReadAsRegexp(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	before := deployment.DeepCopy()

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "/.*/",
		Replicas: int32Ptr(5),
	}})
	require.NoError(t, err)
	require.Equal(t, before, deployment)
}
