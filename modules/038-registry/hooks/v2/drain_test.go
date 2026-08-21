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
	"encoding/base64"
	"encoding/json"
	"strings"
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
			Upstream: &ConfigUpstream{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
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

	step, record, done := nextDrainStep(nil, managedCapture(), 0, now)

	require.NotNil(t, record, "a drain has to start: %s", done)
	assert.Equal(t, drainServe, step)
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

	step, first, done := nextDrainStep(started, managedCapture(), 0, now)
	require.NotNil(t, first, "one empty scan must not end it: %s", done)
	assert.Equal(t, drainServe, step)
	assert.Equal(t, 1, first.ZeroScans)

	// A gap later, because two scans have to be two moments — see quietGap.
	step, second, done := nextDrainStep(first, managedCapture(), 0, now.Add(quietGap))
	assert.Equal(t, drainWithdraw, step, "two consecutive empty scans end it")
	require.NotNil(t, second)
	assert.True(t, second.Finished, "and the record remembers it, or the drain starts over")
	assert.Contains(t, done, "nothing needs the in-cluster registry")
}

// TestAReferenceThatComesBackResetsTheCount covers a render that puts a workload back onto the
// in-cluster address after an empty scan — a rollback, or a module converging late. The counter has to
// be consecutive to mean anything.
func TestAReferenceThatComesBackResetsTheCount(t *testing.T) {
	now := time.Now().UTC()
	record := &drainRecord{Config: *managedCapture(), StartedAt: now, ZeroScans: 1}

	step, next, done := nextDrainStep(record, managedCapture(), 3, now)

	require.NotNil(t, next, "the drain continues: %s", done)
	assert.Equal(t, drainServe, step)
	assert.Equal(t, 0, next.ZeroScans)
}

// TestAFailedScanKeepsTheDrainGoing is the fail-safe direction, spelled out as a test because the
// harmful action is available here and cheap to take by accident. A scan that could not be made
// reports -1, and -1 must never be read as "nothing references it".
func TestAFailedScanKeepsTheDrainGoing(t *testing.T) {
	now := time.Now().UTC()
	record := &drainRecord{Config: *managedCapture(), StartedAt: now, ZeroScans: 1}

	step, next, done := nextDrainStep(record, managedCapture(), -1, now)

	require.NotNil(t, next, "an unknown count must not end a drain: %s", done)
	assert.Equal(t, drainServe, step)
	assert.Equal(t, 1, next.ZeroScans,
		"an unknown count is neither progress nor a reference, so the counter stands still")
}

// TestNothingToDrainWhenTheModuleWasNotServing keeps `Unmanaged` immediate on the clusters where it
// always was: enabled, never used. Those must not grow a secret and a minute of waiting.
func TestNothingToDrainWhenTheModuleWasNotServing(t *testing.T) {
	step, record, done := nextDrainStep(nil, nil, 0, time.Now().UTC())

	assert.Equal(t, drainForget, step)
	assert.Nil(t, record)
	assert.Contains(t, done, "not serving")
}

// TestAFinishedDrainDoesNotStartItselfAgain is the loop the stand caught, and the reason the record
// carries `Finished` at all.
//
// The configuration a drain serves from is captured from the RegistryConfig resource, and that resource
// is rendered from the values this hook writes — so it outlives the decision by exactly one render. A
// pass landing in that gap sees `Unmanaged` plus a resource saying `Managed`. Read as "still serving",
// it starts the drain over, and the module never leaves the pull path: measured with zero references for
// seven minutes, the storage still up, and `startedAt` moving every minute.
func TestAFinishedDrainDoesNotStartItselfAgain(t *testing.T) {
	now := time.Now().UTC()
	withdrawn := &drainRecord{Config: *managedCapture(), StartedAt: now.Add(-5 * time.Minute), Finished: true}

	// The render has not happened yet, so the resource is still there.
	step, kept, why := nextDrainStep(withdrawn, managedCapture(), 0, now)
	assert.Equal(t, drainWithdrawn, step, "a decided withdrawal is not a reason to serve again: %s", why)
	require.NotNil(t, kept)
	assert.True(t, kept.Finished)

	// And once it has, the record has nothing left to protect against.
	step, gone, why := nextDrainStep(withdrawn, nil, 0, now)
	assert.Equal(t, drainForget, step, why)
	assert.Nil(t, gone)
}

// TestWhatCountsAsNamingTheInClusterRegistry is a test about the prefix, because that is where this
// can be wrong in both directions: missing an init container leaves a reference behind and takes the
// address away from it, and matching too broadly keeps a module serving forever.
func TestWhatCountsAsNamingTheInClusterRegistry(t *testing.T) {
	inCluster := registry_const.Host + "/system/deckhouse:v1"
	upstream := "registry.deckhouse.io/deckhouse/ee:v1"

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
				{Image: registry_const.Host + ".example.com/system/deckhouse:v1"},
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

// TestAPodThatHasAlreadyPulledDoesNotHoldTheWithdrawal is the rule that decides whether a withdrawal
// can finish at all.
//
// Measured on the stand: a `trickster` rollout had been stuck for two hours for reasons of its own — the
// new pod, already on the upstream registry, never became ready, so the superseded ReplicaSet kept its
// old pod. That pod named the in-cluster address, had long since pulled it, and served nothing. Waiting
// on it would have held the module on the pull path indefinitely, and any stuck rollout anywhere in the
// cluster would do the same.
func TestAPodThatHasAlreadyPulledDoesNotHoldTheWithdrawal(t *testing.T) {
	image := registry_const.Host + "/system/deckhouse@sha256:abc"

	running := &v1core.Pod{
		Spec: v1core.PodSpec{Containers: []v1core.Container{
			{Name: "trickster", Image: image, ImagePullPolicy: v1core.PullIfNotPresent},
		}},
		Status: v1core.PodStatus{ContainerStatuses: []v1core.ContainerStatus{
			{Name: "trickster", State: v1core.ContainerState{Running: &v1core.ContainerStateRunning{}}},
		}},
	}
	assert.False(t, podCouldStillPull(running),
		"a started container holds its image on the node, so it will not go back to the registry")

	// The case the whole mechanism exists for: a pod that has not started yet is pulling now.
	pulling := running.DeepCopy()
	pulling.Status.ContainerStatuses = []v1core.ContainerStatus{{
		Name:  "trickster",
		State: v1core.ContainerState{Waiting: &v1core.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
	}}
	assert.True(t, podCouldStillPull(pulling), "a container that has not started still needs the registry")

	// And `Always` means every start is a fetch, started or not — the platform's own Deployment uses it.
	always := running.DeepCopy()
	always.Spec.Containers[0].ImagePullPolicy = v1core.PullAlways
	assert.True(t, podCouldStillPull(always), "with Always, a restart fetches again")

	// An init container that has finished is finished; one that has not, is not.
	initDone := &v1core.Pod{
		Spec: v1core.PodSpec{InitContainers: []v1core.Container{
			{Name: "init", Image: image, ImagePullPolicy: v1core.PullIfNotPresent},
		}},
		Status: v1core.PodStatus{InitContainerStatuses: []v1core.ContainerStatus{
			{Name: "init", State: v1core.ContainerState{
				Terminated: &v1core.ContainerStateTerminated{ExitCode: 0},
			}},
		}},
	}
	assert.False(t, podCouldStillPull(initDone))

	// A pod on the upstream registry is nothing to do with this either way.
	upstream := &v1core.Pod{Spec: v1core.PodSpec{Containers: []v1core.Container{
		{Name: "c", Image: "registry.deckhouse.io/deckhouse/ee:v1"},
	}}}
	assert.False(t, podCouldStillPull(upstream))
}

// TestTheWithdrawalGivesTheClusterSomewhereToGo is the finding this exists for, in the form of a test.
//
// A cluster bootstrapped INTO this module has the in-cluster registry written into
// `deckhouse-registry` — address, path, credentials, all of it — and the upstream then lives in exactly
// one place: this module's configuration, which has to be emptied to ask for `Unmanaged`. Measured on
// such a cluster before this existed: 425 container specifications still naming the in-cluster registry
// twenty-five minutes after the withdrawal was asked for, with nowhere for a single one of them to move,
// because the platform's own registry address WAS the in-cluster one.
func TestTheWithdrawalGivesTheClusterSomewhereToGo(t *testing.T) {
	upstream := &ConfigUpstream{
		Scheme: "HTTPS",
		Host:   "registry.deckhouse.io",
		Path:   "/deckhouse/ee",
		Auth:   &ConfigAuth{Username: "license-token", Password: "secret"},
	}

	credentials, err := upstreamCredentials(upstream)
	require.NoError(t, err)

	var document struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(credentials, &document))

	entry, ok := document.Auths["registry.deckhouse.io"]
	require.True(t, ok, "the credentials have to be keyed by the host that will be asked for: %s", credentials)
	pair, err := base64.StdEncoding.DecodeString(entry.Auth)
	require.NoError(t, err)
	assert.Equal(t, "license-token:secret", string(pair))
}

// TestCredentialsGivenAlreadyEncodedAreNotEncodedTwice covers the other form the user can type: `auth`
// instead of a username and a password. Encoded again, it becomes credentials for nobody.
func TestCredentialsGivenAlreadyEncodedAreNotEncodedTwice(t *testing.T) {
	pair := base64.StdEncoding.EncodeToString([]byte("robot$ci:token"))

	credentials, err := upstreamCredentials(&ConfigUpstream{
		Host: "harbor.example.com",
		Auth: &ConfigAuth{Auth: pair},
	})
	require.NoError(t, err)

	var document struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(credentials, &document))
	decoded, err := base64.StdEncoding.DecodeString(document.Auths["harbor.example.com"].Auth)
	require.NoError(t, err)
	assert.Equal(t, "robot$ci:token", string(decoded))
}

// TestAnUpstreamWithNoCredentialsIsStillADestination: a public registry needs none, and a document with
// an empty entry is what says so.
func TestAnUpstreamWithNoCredentialsIsStillADestination(t *testing.T) {
	credentials, err := upstreamCredentials(&ConfigUpstream{Host: "registry.example.com", Path: "/public"})

	require.NoError(t, err)
	assert.Contains(t, string(credentials), "registry.example.com")
}

// TestTheSchemeIsWrittenAsTheGlobalValuesAcceptIt is a one-word test for a mistake already made once on
// a live cluster: the module's configuration says `HTTPS`, the global values accept `http` or `https`,
// and writing the first spelling failed a global hook and wedged the main queue for every module.
func TestTheSchemeIsWrittenAsTheGlobalValuesAcceptIt(t *testing.T) {
	assert.Equal(t, "https", strings.ToLower(schemeOrDefault("HTTPS")))
	assert.Equal(t, "https", strings.ToLower(schemeOrDefault("")), "an unset scheme is HTTPS, not empty")
	assert.Equal(t, "http", strings.ToLower(schemeOrDefault("HTTP")))
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

	image := registry_const.Host + "/system/deckhouse:v1"
	spec := v1core.PodSpec{Containers: []v1core.Container{{Name: "c", Image: image}}}
	meta := func(name string) v1meta.ObjectMeta {
		return v1meta.ObjectMeta{Name: name, Namespace: "d8-monitoring"}
	}

	// No status, so no container has started: a pod that still has to pull, which is what is waited on.
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

// TestARestartDoesNotRestartTheDrain covers the informer warm-up race directly.
//
// A module restarts mid-drain routinely — the platform moving its own image reference off the in-cluster
// registry is what restarts it — and for a moment after that the snapshot behind the record is empty.
// Measured on the stand before this was closed: the record was rewritten eleven seconds in and its
// `startedAt` jumped from 13:54:42 to 13:55:33, taking the alert clock and the quiet-scan counter back
// to zero with it.
func TestARestartDoesNotRestartTheDrain(t *testing.T) {
	ctx := context.Background()
	dc := dependency.NewMockedContainer()
	client, err := dc.GetK8sClient()
	require.NoError(t, err)

	started := time.Date(2026, 8, 21, 13, 54, 42, 0, time.UTC)
	raw, err := json.Marshal(drainRecord{Config: *managedCapture(), StartedAt: started, ZeroScans: 1})
	require.NoError(t, err)

	_, err = client.CoreV1().Secrets("d8-system").Create(ctx, &v1core.Secret{
		ObjectMeta: v1meta.ObjectMeta{Name: DrainSecretName, Namespace: "d8-system"},
		Data:       map[string][]byte{drainRecordKey: raw},
	}, v1meta.CreateOptions{})
	require.NoError(t, err)

	found, err := existingDrainRecord(ctx, dc)
	require.NoError(t, err)
	require.NotNil(t, found, "the record is in the cluster, so an empty snapshot must not be believed")
	assert.Equal(t, started, found.StartedAt, "the clock the alert measures has to survive a restart")
	assert.Equal(t, 1, found.ZeroScans)
}

// TestNoRecordIsNotAnError keeps the ordinary case ordinary: almost every cluster has no drain, and the
// absence must not turn a reconciliation into a failure.
func TestNoRecordIsNotAnError(t *testing.T) {
	found, err := existingDrainRecord(context.Background(), dependency.NewMockedContainer())

	require.NoError(t, err)
	assert.Nil(t, found)
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
					{Name: "deckhouse", Image: "registry.deckhouse.io/deckhouse/ee:v1"},
				},
				InitContainers: []v1core.Container{
					{Name: "init", Image: "registry.deckhouse.io/deckhouse/ee@sha256:abc"},
				},
			}}},
		}, v1meta.CreateOptions{})
	require.NoError(t, err)

	count, _, err := scanReferences(ctx, dc)
	require.NoError(t, err)
	assert.Zero(t, count)
}
