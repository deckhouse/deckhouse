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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/werf/nelm/v2/pkg/resource/spec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const releaseNamespace = "d8-test"

// apply runs the requests' patches over resources the way nelm does on render,
// replacing each resource's content with the patched one, and returns the first
// error building or applying them.
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
		if err != nil {
			return err
		}

		resource.Object = patched.Object
	}

	return nil
}

// parse turns a manifest into the unstructured shape nelm patches. It decodes as
// nelm does, integers as int64, the type they come back from a patch in, so a
// patch that runs over an object without changing it leaves it equal.
func parse(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()

	data, err := yaml.YAMLToJSON([]byte(manifest))
	require.NoError(t, err)

	resource := new(unstructured.Unstructured)
	require.NoError(t, resource.UnmarshalJSON(data))

	return resource
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

const statefulSetManifest = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: postgres
          image: postgres:v1
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 10Gi
    - metadata:
        name: wal
      spec:
        accessModes: [ReadWriteOnce]
`

// templateStorage digs out the storage a volume claim template of the
// StatefulSet requests.
func templateStorage(t *testing.T, statefulSet *unstructured.Unstructured, name string) string {
	t.Helper()

	templates, found, err := unstructured.NestedSlice(statefulSet.Object, "spec", "volumeClaimTemplates")
	require.NoError(t, err)
	require.True(t, found)

	for _, entry := range templates {
		template, ok := entry.(map[string]any)
		require.True(t, ok)

		if templateName, _, _ := unstructured.NestedString(template, "metadata", "name"); templateName == name {
			storage, _, err := unstructured.NestedString(template, "spec", "resources", "requests", "storage")
			require.NoError(t, err)

			return storage
		}
	}

	t.Fatalf("volume claim template %q not found", name)

	return ""
}

func TestPatchesResizesVolumeClaimTemplates(t *testing.T) {
	statefulSet := parse(t, statefulSetManifest)

	err := apply(t, []*unstructured.Unstructured{statefulSet}, []Request{{
		Kind: "StatefulSet",
		Name: "postgres",
		VolumeClaimTemplates: []VolumeClaimTemplate{
			{Name: "data", Storage: "100Gi"},
			// The chart's template requests no storage; the patch writes it.
			{Name: "wal", Storage: "20Gi"},
			// A template the StatefulSet does not have is ignored, not created.
			{Name: "absent", Storage: "5Gi"},
		},
	}})
	require.NoError(t, err)

	require.Equal(t, "100Gi", templateStorage(t, statefulSet, "data"))
	require.Equal(t, "20Gi", templateStorage(t, statefulSet, "wal"))

	templates, _, err := unstructured.NestedSlice(statefulSet.Object, "spec", "volumeClaimTemplates")
	require.NoError(t, err)
	require.Len(t, templates, 2)
}

// The templates are resized in the same program as the replicas and the
// containers of the StatefulSet, so each step must leave the next its input.
func TestPatchesResizeTemplatesAlongWithTheWorkload(t *testing.T) {
	statefulSet := parse(t, statefulSetManifest)

	err := apply(t, []*unstructured.Unstructured{statefulSet}, []Request{{
		Kind:                 "StatefulSet",
		Name:                 "postgres",
		Replicas:             int32Ptr(3),
		Containers:           []Container{{Name: "postgres", Limits: map[string]string{"memory": "1Gi"}}},
		VolumeClaimTemplates: []VolumeClaimTemplate{{Name: "data", Storage: "100Gi"}},
	}})
	require.NoError(t, err)

	replicas, _, err := unstructured.NestedInt64(statefulSet.Object, "spec", "replicas")
	require.NoError(t, err)
	require.Equal(t, int64(3), replicas)

	resources := containerResources(t, statefulSet, "postgres", "spec", "template", "spec")
	require.Equal(t, map[string]any{"memory": "1Gi"}, resources["limits"])

	require.Equal(t, "100Gi", templateStorage(t, statefulSet, "data"))
}

// The CR schema rejects volume claim templates on any kind but StatefulSet; the
// overlay refuses them too, like replicas.
func TestPatchesRejectVolumeClaimTemplatesOfOtherKinds(t *testing.T) {
	err := apply(t, []*unstructured.Unstructured{parse(t, deploymentManifest)}, []Request{{
		Kind:                 "Deployment",
		Name:                 "controller",
		VolumeClaimTemplates: []VolumeClaimTemplate{{Name: "data", Storage: "1Gi"}},
	}})
	require.ErrorContains(t, err, "volume claim templates")
}

// The checksum of the accepted configuration is built from the encoded requests,
// so a request that does not use the disk field must encode exactly as it did
// before it existed: otherwise every Application with resource requests would
// see its configuration change on upgrade and run again.
func TestRequestWithoutDisksEncodesAsBefore(t *testing.T) {
	encoded, err := json.Marshal(Request{
		Kind:       "Deployment",
		Name:       "controller",
		Replicas:   int32Ptr(2),
		Containers: []Container{{Name: "controller", Limits: map[string]string{"cpu": "1"}}},
	})
	require.NoError(t, err)
	require.Equal(t, `{"kind":"Deployment","name":"controller","replicas":2,"containers":[{"name":"controller","limits":{"cpu":"1"}}]}`, string(encoded))
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

// scalableDeployment renders a Deployment with the given replicas line and
// annotations, so the scale guards can be checked against the chart's value.
func scalableDeployment(t *testing.T, replicas string, annotations map[string]string) *unstructured.Unstructured {
	t.Helper()

	deployment := parse(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller
spec:
  `+replicas+`
  template:
    spec:
      containers:
        - name: controller
`)
	deployment.SetAnnotations(annotations)

	return deployment
}

// renderedReplicas reads spec.replicas of a patched workload.
func renderedReplicas(t *testing.T, resource *unstructured.Unstructured) int64 {
	t.Helper()

	replicas, found, err := unstructured.NestedInt64(resource.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)

	return replicas
}

// requireScalingError asserts err carries a scaling error with message, as ScalingError finds it
// through nelm's wrapping.
func requireScalingError(t *testing.T, err error, message string) {
	t.Helper()

	require.Error(t, err)
	require.Equal(t, message, ScalingError(fmt.Errorf("render chart: %w", err)))
}

func TestScalingErrorIgnoresOtherErrors(t *testing.T) {
	require.Empty(t, ScalingError(errors.New("boom")))
	require.Empty(t, ScalingError(nil))
}

func TestPatchesRefuseScalingWhenScaleDisabled(t *testing.T) {
	for _, requested := range []int32{1, 5} {
		deployment := scalableDeployment(t, "replicas: 3", map[string]string{annotationScaleDisabled: "true"})

		err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
			Kind:     "Deployment",
			Name:     "controller",
			Replicas: int32Ptr(requested),
		}})
		requireScalingError(t, err, "Deployment 'controller' does not allow its replicas to change")
	}
}

func TestPatchesRefuseScalingDownWhenScaleDownDisabled(t *testing.T) {
	deployment := scalableDeployment(t, "replicas: 3", map[string]string{annotationScaleDownDisabled: "true"})

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(1),
	}})
	requireScalingError(t, err, "Deployment 'controller' does not allow its replicas to go below 3")
}

func TestPatchesScaleUpWhenScaleDownDisabled(t *testing.T) {
	deployment := scalableDeployment(t, "replicas: 3", map[string]string{annotationScaleDownDisabled: "true"})

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(5),
	}})
	require.NoError(t, err)
	require.Equal(t, int64(5), renderedReplicas(t, deployment))
}

// Asking for the chart's own value changes nothing, so neither guard objects.
func TestPatchesKeepChartReplicasUnderGuards(t *testing.T) {
	for _, annotation := range []string{annotationScaleDisabled, annotationScaleDownDisabled} {
		deployment := scalableDeployment(t, "replicas: 3", map[string]string{annotation: "true"})

		err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
			Kind:     "Deployment",
			Name:     "controller",
			Replicas: int32Ptr(3),
		}})
		require.NoError(t, err)
		require.Equal(t, int64(3), renderedReplicas(t, deployment))
	}
}

// Only the exact value "true" arms a guard.
func TestPatchesIgnoreGuardsNotSetToTrue(t *testing.T) {
	deployment := scalableDeployment(t, "replicas: 3", map[string]string{
		annotationScaleDisabled:     "false",
		annotationScaleDownDisabled: "True",
	})

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(0),
	}})
	require.NoError(t, err)
	require.Equal(t, int64(0), renderedReplicas(t, deployment))
}

// A chart that leaves replicas unset runs one, as Kubernetes defaults it.
func TestPatchesTreatUnsetReplicasAsOne(t *testing.T) {
	deployment := scalableDeployment(t, "", map[string]string{annotationScaleDownDisabled: "true"})

	err := apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(0),
	}})
	requireScalingError(t, err, "Deployment 'controller' does not allow its replicas to go below 1")

	deployment = scalableDeployment(t, "", map[string]string{annotationScaleDownDisabled: "true"})

	err = apply(t, []*unstructured.Unstructured{deployment}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(1),
	}})
	require.NoError(t, err)
	require.Equal(t, int64(1), renderedReplicas(t, deployment))
}

// hpa renders a HorizontalPodAutoscaler of apiVersion that scales the workload of
// kind and name between 2 and 10 replicas.
func hpa(t *testing.T, apiVersion, kind, name string) *unstructured.Unstructured {
	t.Helper()

	return parse(t, fmt.Sprintf(`
apiVersion: %s
kind: HorizontalPodAutoscaler
metadata:
  name: autoscaler
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: %s
    name: %s
  minReplicas: 2
  maxReplicas: 10
`, apiVersion, kind, name))
}

// hpaReplicas reads minReplicas and maxReplicas of a patched HorizontalPodAutoscaler.
func hpaReplicas(t *testing.T, resource *unstructured.Unstructured) (int64, int64) {
	t.Helper()

	minReplicas, found, err := unstructured.NestedInt64(resource.Object, "spec", "minReplicas")
	require.NoError(t, err)
	require.True(t, found)

	maxReplicas, found, err := unstructured.NestedInt64(resource.Object, "spec", "maxReplicas")
	require.NoError(t, err)
	require.True(t, found)

	return minReplicas, maxReplicas
}

// vpa renders a VerticalPodAutoscaler for the workload of kind and name with the
// given resource policy, none when it is empty.
func vpa(t *testing.T, kind, name, resourcePolicy string) *unstructured.Unstructured {
	t.Helper()

	return parse(t, fmt.Sprintf(`
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: autoscaler
spec:
  targetRef:
    apiVersion: apps/v1
    kind: %s
    name: %s
  updatePolicy:
    updateMode: InPlaceOrRecreate
%s`, kind, name, resourcePolicy))
}

// controllerPolicies bounds both containers of deploymentManifest's workload.
const controllerPolicies = `
  resourcePolicy:
    containerPolicies:
      - containerName: controller
        minAllowed:
          cpu: 10m
          memory: 32Mi
        maxAllowed:
          cpu: 500m
          memory: 512Mi
      - containerName: kube-rbac-proxy
        minAllowed:
          cpu: 5m
          memory: 16Mi
        maxAllowed:
          cpu: 50m
          memory: 64Mi
`

// vpaPolicies reads the container policies of a patched VerticalPodAutoscaler.
func vpaPolicies(t *testing.T, resource *unstructured.Unstructured) []any {
	t.Helper()

	policies, found, err := unstructured.NestedSlice(resource.Object, "spec", "resourcePolicy", "containerPolicies")
	require.NoError(t, err)
	require.True(t, found)

	return policies
}

// vpaPolicy finds the policy of the container in a patched VerticalPodAutoscaler.
func vpaPolicy(t *testing.T, resource *unstructured.Unstructured, container string) map[string]any {
	t.Helper()

	for _, entry := range vpaPolicies(t, resource) {
		policy, ok := entry.(map[string]any)
		require.True(t, ok)

		if policy["containerName"] == container {
			return policy
		}
	}

	t.Fatalf("policy of container %q not found", container)

	return nil
}

// Both API versions of HorizontalPodAutoscaler hold the replicas in the same fields.
func TestPatchesPinHorizontalPodAutoscalers(t *testing.T) {
	for _, apiVersion := range []string{"autoscaling/v1", "autoscaling/v2"} {
		deployment := parse(t, deploymentManifest)
		autoscaler := hpa(t, apiVersion, "Deployment", "controller")

		err := apply(t, []*unstructured.Unstructured{deployment, autoscaler}, []Request{{
			Kind:     "Deployment",
			Name:     "controller",
			Replicas: int32Ptr(5),
		}})
		require.NoError(t, err)

		minReplicas, maxReplicas := hpaReplicas(t, autoscaler)
		require.Equal(t, int64(5), minReplicas, apiVersion)
		require.Equal(t, int64(5), maxReplicas, apiVersion)
		require.Equal(t, int64(5), renderedReplicas(t, deployment), apiVersion)
	}
}

// A HorizontalPodAutoscaler cannot hold zero replicas; the workload alone takes
// them, and the autoscaler does not scale a target that runs none.
func TestPatchesLeaveHorizontalPodAutoscalerOutOfZeroReplicas(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	autoscaler := hpa(t, "autoscaling/v2", "Deployment", "controller")
	before := autoscaler.DeepCopy()

	err := apply(t, []*unstructured.Unstructured{deployment, autoscaler}, []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(0),
	}})
	require.NoError(t, err)

	require.Equal(t, before, autoscaler)
	require.Equal(t, int64(0), renderedReplicas(t, deployment))
}

// An autoscaler is pinned only when it targets the requested workload by both kind
// and name, and only when it is of the autoscaling API group.
func TestPatchesLeaveOtherAutoscalersAlone(t *testing.T) {
	foreignVPA := vpa(t, "Deployment", "controller", controllerPolicies)
	foreignVPA.SetAPIVersion("autoscaling.example.com/v1")

	others := []*unstructured.Unstructured{
		hpa(t, "autoscaling/v2", "Deployment", "webhook"),
		hpa(t, "autoscaling/v2", "StatefulSet", "controller"),
		hpa(t, "autoscaling.example.com/v1", "Deployment", "controller"),
		vpa(t, "Deployment", "webhook", controllerPolicies),
		vpa(t, "StatefulSet", "controller", controllerPolicies),
		foreignVPA,
	}

	before := make([]*unstructured.Unstructured, 0, len(others))
	for _, other := range others {
		before = append(before, other.DeepCopy())
	}

	err := apply(t, append([]*unstructured.Unstructured{parse(t, deploymentManifest)}, others...), []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(3),
		Containers: []Container{{
			Name:     "controller",
			Requests: map[string]string{"cpu": "100m"},
			Limits:   map[string]string{"memory": "1Gi"},
		}},
	}})
	require.NoError(t, err)
	require.Equal(t, before, others)
}

// The requested cpu and memory become both bounds of the container's policy, so
// VPA keeps them; the bounds of everything else stay as the chart sets them.
func TestPatchesPinVerticalPodAutoscalerRequests(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	autoscaler := vpa(t, "Deployment", "controller", controllerPolicies)
	proxyPolicy := vpaPolicy(t, autoscaler.DeepCopy(), "kube-rbac-proxy")

	err := apply(t, []*unstructured.Unstructured{deployment, autoscaler}, []Request{{
		Kind: "Deployment",
		Name: "controller",
		Containers: []Container{{
			Name:     "controller",
			Requests: map[string]string{"memory": "1Gi", "ephemeral-storage": "2Gi"},
		}},
	}})
	require.NoError(t, err)

	// VPA does not control ephemeral-storage: it goes to the workload alone.
	require.Equal(t, map[string]any{
		"containerName": "controller",
		"minAllowed":    map[string]any{"cpu": "10m", "memory": "1Gi"},
		"maxAllowed":    map[string]any{"cpu": "500m", "memory": "1Gi"},
	}, vpaPolicy(t, autoscaler, "controller"))
	require.Equal(t, proxyPolicy, vpaPolicy(t, autoscaler, "kube-rbac-proxy"))

	resources := containerResources(t, deployment, "controller", "spec", "template", "spec")
	require.Equal(t, map[string]any{"cpu": "10m", "memory": "1Gi", "ephemeral-storage": "2Gi"}, resources["requests"])
}

// VPA scales a limit together with its request, so a container that sets a cpu or
// memory limit makes VPA leave its limits as the workload declares them.
func TestPatchesKeepLimitsOutOfVerticalPodAutoscaler(t *testing.T) {
	autoscaler := vpa(t, "Deployment", "controller", controllerPolicies)
	before := autoscaler.DeepCopy()

	err := apply(t, []*unstructured.Unstructured{parse(t, deploymentManifest), autoscaler}, []Request{{
		Kind: "Deployment",
		Name: "controller",
		Containers: []Container{
			{Name: "controller", Limits: map[string]string{"cpu": "1"}},
			{Name: "kube-rbac-proxy", Limits: map[string]string{"ephemeral-storage": "1Gi"}},
		},
	}})
	require.NoError(t, err)

	controllerPolicy := vpaPolicy(t, before, "controller")
	controllerPolicy["controlledValues"] = "RequestsOnly"

	require.Equal(t, controllerPolicy, vpaPolicy(t, autoscaler, "controller"))
	require.Equal(t, vpaPolicy(t, before, "kube-rbac-proxy"), vpaPolicy(t, autoscaler, "kube-rbac-proxy"))
}

// A container that VPA takes by the "*" policy gets a copy of it to pin, so the
// other containers keep following "*" unchanged.
func TestPatchesCopyDefaultVerticalPodAutoscalerPolicy(t *testing.T) {
	autoscaler := vpa(t, "Deployment", "controller", `
  resourcePolicy:
    containerPolicies:
      - containerName: "*"
        controlledValues: RequestsAndLimits
        minAllowed:
          cpu: 10m
          memory: 32Mi
        maxAllowed:
          cpu: 500m
          memory: 512Mi
`)
	defaultPolicy := vpaPolicy(t, autoscaler.DeepCopy(), "*")

	err := apply(t, []*unstructured.Unstructured{parse(t, deploymentManifest), autoscaler}, []Request{{
		Kind:       "Deployment",
		Name:       "controller",
		Containers: []Container{{Name: "controller", Requests: map[string]string{"cpu": "200m"}}},
	}})
	require.NoError(t, err)

	require.Len(t, vpaPolicies(t, autoscaler), 2)
	require.Equal(t, defaultPolicy, vpaPolicy(t, autoscaler, "*"))
	require.Equal(t, map[string]any{
		"containerName":    "controller",
		"controlledValues": "RequestsAndLimits",
		"minAllowed":       map[string]any{"cpu": "200m", "memory": "32Mi"},
		"maxAllowed":       map[string]any{"cpu": "200m", "memory": "512Mi"},
	}, vpaPolicy(t, autoscaler, "controller"))
}

// A VerticalPodAutoscaler without a resource policy gets one for the container.
func TestPatchesAddVerticalPodAutoscalerPolicy(t *testing.T) {
	autoscaler := vpa(t, "Deployment", "controller", "")

	err := apply(t, []*unstructured.Unstructured{parse(t, deploymentManifest), autoscaler}, []Request{{
		Kind: "Deployment",
		Name: "controller",
		Containers: []Container{{
			Name:     "controller",
			Requests: map[string]string{"memory": "256Mi"},
			Limits:   map[string]string{"memory": "512Mi"},
		}},
	}})
	require.NoError(t, err)

	require.Equal(t, []any{map[string]any{
		"containerName":    "controller",
		"controlledValues": "RequestsOnly",
		"minAllowed":       map[string]any{"memory": "256Mi"},
		"maxAllowed":       map[string]any{"memory": "256Mi"},
	}}, vpaPolicies(t, autoscaler))
}

// VPA rejects a bound finer than a whole milli CPU or a whole byte, which a pod
// takes, so the bound is rounded up while the workload keeps the exact request.
func TestPatchesRoundVerticalPodAutoscalerBoundsUp(t *testing.T) {
	deployment := parse(t, deploymentManifest)
	autoscaler := vpa(t, "Deployment", "controller", controllerPolicies)

	err := apply(t, []*unstructured.Unstructured{deployment, autoscaler}, []Request{{
		Kind:       "Deployment",
		Name:       "controller",
		Containers: []Container{{Name: "controller", Requests: map[string]string{"cpu": "0.5m", "memory": "1.2Gi"}}},
	}})
	require.NoError(t, err)

	policy := vpaPolicy(t, autoscaler, "controller")
	require.Equal(t, map[string]any{"cpu": "1m", "memory": "1288490189"}, policy["minAllowed"])
	require.Equal(t, map[string]any{"cpu": "1m", "memory": "1288490189"}, policy["maxAllowed"])

	resources := containerResources(t, deployment, "controller", "spec", "template", "spec")
	require.Equal(t, map[string]any{"cpu": "0.5m", "memory": "1.2Gi"}, resources["requests"])
}

// A request that bypassed the CR schema with a malformed quantity fails the
// patches rather than reaching a VerticalPodAutoscaler.
func TestPatchesRejectMalformedVerticalPodAutoscalerRequest(t *testing.T) {
	_, err := Patches([]Request{{
		Kind:       "Deployment",
		Name:       "controller",
		Containers: []Container{{Name: "controller", Requests: map[string]string{"memory": "lots"}}},
	}})
	require.ErrorContains(t, err, "container 'controller' memory request")
}

// scaleController asks for the replicas of deploymentManifest's workload, rendered
// together with the autoscalers.
func scaleController(t *testing.T, replicas int32, autoscalers ...*unstructured.Unstructured) error {
	t.Helper()

	return apply(t, append([]*unstructured.Unstructured{parse(t, deploymentManifest)}, autoscalers...), []Request{{
		Kind:     "Deployment",
		Name:     "controller",
		Replicas: int32Ptr(replicas),
	}})
}

// guardedHPA renders the HorizontalPodAutoscaler of deploymentManifest's workload,
// scaling it between 2 and 10 replicas, with the annotations.
func guardedHPA(t *testing.T, annotations map[string]string) *unstructured.Unstructured {
	t.Helper()

	autoscaler := hpa(t, "autoscaling/v2", "Deployment", "controller")
	autoscaler.SetAnnotations(annotations)

	return autoscaler
}

// requirePinned asserts a patched HorizontalPodAutoscaler holds replicas as both bounds.
func requirePinned(t *testing.T, autoscaler *unstructured.Unstructured, replicas int64) {
	t.Helper()

	minReplicas, maxReplicas := hpaReplicas(t, autoscaler)
	require.Equal(t, replicas, minReplicas)
	require.Equal(t, replicas, maxReplicas)
}

// Pinning both bounds changes them unless both hold the replicas already: 2 is the
// minimum but not the maximum. Zero leaves the autoscaler as it is but stops its
// target, which changes the replicas all the same.
func TestPatchesRefuseScalingWhenHorizontalPodAutoscalerScaleDisabled(t *testing.T) {
	for _, requested := range []int32{0, 2, 5} {
		err := scaleController(t, requested, guardedHPA(t, map[string]string{annotationScaleDisabled: "true"}))
		requireScalingError(t, err, "HorizontalPodAutoscaler 'autoscaler' does not allow the replicas of Deployment 'controller' to change")
	}
}

func TestPatchesRefuseScalingBelowHorizontalPodAutoscalerMinimum(t *testing.T) {
	for _, requested := range []int32{0, 1} {
		err := scaleController(t, requested, guardedHPA(t, map[string]string{annotationScaleDownDisabled: "true"}))
		requireScalingError(t, err, "HorizontalPodAutoscaler 'autoscaler' does not allow the replicas of Deployment 'controller' to go below 2")
	}
}

// Scaling down disabled still lets the replicas stay at the minimum or rise past the maximum.
func TestPatchesPinHorizontalPodAutoscalerWhenScaleDownDisabled(t *testing.T) {
	for _, requested := range []int32{2, 15} {
		autoscaler := guardedHPA(t, map[string]string{annotationScaleDownDisabled: "true"})

		require.NoError(t, scaleController(t, requested, autoscaler))
		requirePinned(t, autoscaler, int64(requested))
	}
}

// Asking for the replicas both bounds already hold changes nothing, so neither guard objects.
func TestPatchesKeepHorizontalPodAutoscalerBoundsUnderGuards(t *testing.T) {
	for _, annotation := range []string{annotationScaleDisabled, annotationScaleDownDisabled} {
		autoscaler := guardedHPA(t, map[string]string{annotation: "true"})
		require.NoError(t, unstructured.SetNestedField(autoscaler.Object, int64(3), "spec", "minReplicas"))
		require.NoError(t, unstructured.SetNestedField(autoscaler.Object, int64(3), "spec", "maxReplicas"))

		require.NoError(t, scaleController(t, 3, autoscaler))
		requirePinned(t, autoscaler, 3)
	}
}

// An autoscaler that leaves minReplicas unset keeps one replica, as Kubernetes defaults it.
func TestPatchesTreatUnsetMinReplicasAsOne(t *testing.T) {
	autoscaler := guardedHPA(t, map[string]string{annotationScaleDownDisabled: "true"})
	unstructured.RemoveNestedField(autoscaler.Object, "spec", "minReplicas")

	err := scaleController(t, 0, autoscaler)
	requireScalingError(t, err, "HorizontalPodAutoscaler 'autoscaler' does not allow the replicas of Deployment 'controller' to go below 1")

	require.NoError(t, scaleController(t, 1, autoscaler))
	requirePinned(t, autoscaler, 1)
}

// Only the exact value "true" arms a guard.
func TestPatchesIgnoreHorizontalPodAutoscalerGuardsNotSetToTrue(t *testing.T) {
	autoscaler := guardedHPA(t, map[string]string{
		annotationScaleDisabled:     "false",
		annotationScaleDownDisabled: "True",
	})

	require.NoError(t, scaleController(t, 1, autoscaler))
	requirePinned(t, autoscaler, 1)
}

// A guard binds only the replicas of the workload its autoscaler targets.
func TestPatchesIgnoreGuardsOfOtherHorizontalPodAutoscalers(t *testing.T) {
	other := hpa(t, "autoscaling/v2", "Deployment", "webhook")
	other.SetAnnotations(map[string]string{annotationScaleDisabled: "true", annotationScaleDownDisabled: "true"})
	autoscaler := hpa(t, "autoscaling/v2", "Deployment", "controller")

	require.NoError(t, scaleController(t, 1, other, autoscaler))
	requirePinned(t, autoscaler, 1)
}
