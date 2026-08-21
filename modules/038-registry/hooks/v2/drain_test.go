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

package v2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func managedCapture() *RegistryConfig {
	return &RegistryConfig{
		Mode: string(registryv1alpha1.ModeManaged),
		Primary: ConfigPrimary{
			Upstream: &ConfigUpstream{Host: "dev-registry.deckhouse.io", Path: "/sys/deckhouse-oss"},
		},
		Storage: ConfigStorage{Cache: true},
	}
}

// TestADrainStartsEvenWhenTheFirstScanIsEmpty is the case that decides whether this is worth having.
//
// The user asks for `Unmanaged`, and at that instant a scan can easily come back empty — the platform's
// own references live in Deployments that are about to be re-rendered, and a scan landing between two
// renders sees a clean cluster. Ending there is the old behaviour, which cost 84 seconds of the
// platform naming a registry that no longer existed and 680 seconds of workloads unable to pull.
func TestADrainStartsEvenWhenTheFirstScanIsEmpty(t *testing.T) {
	now := time.Now().UTC()

	record, done := nextDrainRecord(nil, managedCapture(), 0, now)

	require.NotNil(t, record, "a drain has to start: %s", done)
	assert.Empty(t, done)
	assert.Equal(t, 0, record.ZeroScans, "the first pass counts as nothing observed yet")
	assert.Equal(t, now, record.StartedAt)
	assert.Equal(t, managedCapture().Primary.Upstream.Host, record.Config.Primary.Upstream.Host,
		"the configuration being served has to be the one that was in effect")
}

// TestADrainNeedsTwoEmptyScansToFinish is about the same window from the other side: one empty scan is
// not proof, two consecutive ones are enough.
func TestADrainNeedsTwoEmptyScansToFinish(t *testing.T) {
	now := time.Now().UTC()
	started := &drainRecord{Config: *managedCapture(), StartedAt: now.Add(-2 * time.Minute)}

	first, done := nextDrainRecord(started, managedCapture(), 0, now)
	require.NotNil(t, first, "one empty scan must not end it: %s", done)
	assert.Equal(t, 1, first.ZeroScans)

	// A gap later, because two scans have to be two moments — see quietGap.
	second, done := nextDrainRecord(first, managedCapture(), 0, now.Add(quietGap))
	assert.Nil(t, second, "two consecutive empty scans end it")
	assert.Contains(t, done, "nothing names the in-cluster registry")
}

// TestAReferenceThatComesBackResetsTheCount covers a render that puts a workload back onto the
// in-cluster address after an empty scan — a rollback, or a module converging late. The counter has to
// be consecutive to mean anything.
func TestAReferenceThatComesBackResetsTheCount(t *testing.T) {
	now := time.Now().UTC()
	record := &drainRecord{Config: *managedCapture(), StartedAt: now, ZeroScans: 1}

	next, done := nextDrainRecord(record, managedCapture(), 3, now)

	require.NotNil(t, next, "the drain continues: %s", done)
	assert.Equal(t, 0, next.ZeroScans)
}

// TestAFailedScanKeepsTheDrainGoing is the fail-safe direction, spelled out as a test because the
// harmful action is available here and cheap to take by accident. A scan that could not be made
// reports -1, and -1 must never be read as "nothing references it".
func TestAFailedScanKeepsTheDrainGoing(t *testing.T) {
	now := time.Now().UTC()
	record := &drainRecord{Config: *managedCapture(), StartedAt: now, ZeroScans: 1}

	next, done := nextDrainRecord(record, managedCapture(), -1, now)

	require.NotNil(t, next, "an unknown count must not end a drain: %s", done)
	assert.Equal(t, 1, next.ZeroScans,
		"an unknown count is neither progress nor a reference, so the counter stands still")
}

// TestNothingToDrainWhenTheModuleWasNotServing keeps `Unmanaged` immediate on the clusters where it
// always was: enabled, never used. Those must not grow a secret and a minute of waiting.
func TestNothingToDrainWhenTheModuleWasNotServing(t *testing.T) {
	record, done := nextDrainRecord(nil, nil, 0, time.Now().UTC())

	assert.Nil(t, record)
	assert.Contains(t, done, "not serving")
}

// TestWhatCountsAsNamingTheInClusterRegistry is a test about the prefix, because that is where this
// can be wrong in both directions: missing an init container leaves a reference behind and takes the
// address away from it, and matching too broadly keeps a module serving forever.
func TestWhatCountsAsNamingTheInClusterRegistry(t *testing.T) {
	inCluster := registry_const.Host + "/system/deckhouse:pr21788"
	upstream := "dev-registry.deckhouse.io/sys/deckhouse-oss:pr21788"

	cases := []struct {
		name  string
		spec  v1core.PodSpec
		names bool
	}{
		{
			name:  "a container",
			spec:  v1core.PodSpec{Containers: []v1core.Container{{Image: inCluster}}},
			names: true,
		},
		{
			// The reference that stranded a cluster on the stand: the platform's Deployment names the
			// in-cluster registry in `init-downloaded-modules`, and patching only the main container
			// left the pod in Init:ImagePullBackOff.
			name:  "an init container",
			spec:  v1core.PodSpec{InitContainers: []v1core.Container{{Image: inCluster}}},
			names: true,
		},
		{
			name: "an ephemeral container",
			spec: v1core.PodSpec{EphemeralContainers: []v1core.EphemeralContainer{{
				EphemeralContainerCommon: v1core.EphemeralContainerCommon{Image: inCluster},
			}}},
			names: true,
		},
		{
			name:  "the upstream registry",
			spec:  v1core.PodSpec{Containers: []v1core.Container{{Image: upstream}}},
			names: false,
		},
		{
			// A host that merely starts with the same characters is a different registry, and reading it
			// as ours would keep the module serving an address nobody asked it to.
			name: "a host that only looks like it",
			spec: v1core.PodSpec{Containers: []v1core.Container{
				{Image: registry_const.Host + ".example.com/system/deckhouse:pr21788"},
			}},
			names: false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.names, namesInClusterRegistry(&test.spec))
		})
	}
}

// TestWhichNamespacesAreScanned records the scope, which is a deliberate limit rather than an
// oversight: the references being waited on come from Deckhouse renders.
func TestWhichNamespacesAreScanned(t *testing.T) {
	assert.True(t, platformNamespace("d8-system"))
	assert.True(t, platformNamespace("d8-monitoring"))
	assert.True(t, platformNamespace("kube-system"), "control-plane static pods are mirrored here")
	assert.False(t, platformNamespace("default"),
		"a workload of the user's that hardcodes the address is what the alert is for, not what the "+
			"withdrawal waits on")
}

// TestTheScanFindsEveryKindThatCanRecreateAPod is the coverage test for the list of kinds. A pod that
// is missed loses its registry; a controller that is missed re-creates the pod after the registry is
// gone, which is worse because it happens later.
func TestTheScanFindsEveryKindThatCanRecreateAPod(t *testing.T) {
	ctx := context.Background()
	// A fresh mocked container per test: it holds a fake cluster, and one shared between tests would
	// make the count depend on what ran before it.
	dc := dependency.NewMockedContainer()
	client, err := dc.GetK8sClient()
	require.NoError(t, err)

	image := registry_const.Host + "/system/deckhouse:pr21788"
	spec := v1core.PodSpec{Containers: []v1core.Container{{Name: "c", Image: image}}}
	meta := func(name string) v1meta.ObjectMeta {
		return v1meta.ObjectMeta{Name: name, Namespace: "d8-monitoring"}
	}

	_, err = client.CoreV1().Pods("d8-monitoring").Create(ctx,
		&v1core.Pod{ObjectMeta: meta("grafana"), Spec: spec}, v1meta.CreateOptions{})
	require.NoError(t, err)
	_, err = client.AppsV1().Deployments("d8-monitoring").Create(ctx,
		&appsv1.Deployment{ObjectMeta: meta("grafana"), Spec: appsv1.DeploymentSpec{
			Template: v1core.PodTemplateSpec{Spec: spec},
		}}, v1meta.CreateOptions{})
	require.NoError(t, err)
	_, err = client.AppsV1().DaemonSets("d8-monitoring").Create(ctx,
		&appsv1.DaemonSet{ObjectMeta: meta("node-exporter"), Spec: appsv1.DaemonSetSpec{
			Template: v1core.PodTemplateSpec{Spec: spec},
		}}, v1meta.CreateOptions{})
	require.NoError(t, err)
	_, err = client.AppsV1().StatefulSets("d8-monitoring").Create(ctx,
		&appsv1.StatefulSet{ObjectMeta: meta("prometheus"), Spec: appsv1.StatefulSetSpec{
			Template: v1core.PodTemplateSpec{Spec: spec},
		}}, v1meta.CreateOptions{})
	require.NoError(t, err)
	_, err = client.BatchV1().CronJobs("d8-monitoring").Create(ctx,
		&batchv1.CronJob{ObjectMeta: meta("cleanup"), Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{
				Template: v1core.PodTemplateSpec{Spec: spec},
			}},
		}}, v1meta.CreateOptions{})
	require.NoError(t, err)

	// And one outside the scanned namespaces, which must not hold a withdrawal open.
	_, err = client.CoreV1().Pods("default").Create(ctx,
		&v1core.Pod{
			ObjectMeta: v1meta.ObjectMeta{Name: "mine", Namespace: "default"},
			Spec:       spec,
		}, v1meta.CreateOptions{})
	require.NoError(t, err)

	count, examples, err := scanReferences(ctx, dc)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "one of each kind in a platform namespace, and nothing from default")
	assert.Len(t, examples, 5)
}

// TestTheScanIsEmptyOnceEverythingMoved is the other end: the state the withdrawal is waiting for has
// to actually be reachable, and an image on the upstream registry must not be counted.
func TestTheScanIsEmptyOnceEverythingMoved(t *testing.T) {
	ctx := context.Background()
	// A fresh mocked container per test: it holds a fake cluster, and one shared between tests would
	// make the count depend on what ran before it.
	dc := dependency.NewMockedContainer()
	client, err := dc.GetK8sClient()
	require.NoError(t, err)

	_, err = client.AppsV1().Deployments("d8-system").Create(ctx,
		&appsv1.Deployment{
			ObjectMeta: v1meta.ObjectMeta{Name: "deckhouse", Namespace: "d8-system"},
			Spec: appsv1.DeploymentSpec{Template: v1core.PodTemplateSpec{Spec: v1core.PodSpec{
				Containers: []v1core.Container{
					{Name: "deckhouse", Image: "dev-registry.deckhouse.io/sys/deckhouse-oss:pr21788"},
				},
				InitContainers: []v1core.Container{
					{Name: "init", Image: "dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:abc"},
				},
			}}},
		}, v1meta.CreateOptions{})
	require.NoError(t, err)

	count, _, err := scanReferences(ctx, dc)
	require.NoError(t, err)
	assert.Zero(t, count)
}
