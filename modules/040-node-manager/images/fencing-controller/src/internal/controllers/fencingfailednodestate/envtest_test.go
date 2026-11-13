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

package fencingfailednodestate

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/adapters/fencingstate"
	"fencing-controller/internal/common"
	"fencing-controller/internal/testenv"
	"fencing-controller/internal/usecase/noderef"
	"fencing-controller/internal/usecase/profile"
)

// The tests below run the reconciler with the real adapters against a real API
// server. The API server has the CRDs and the built-in profiles of the module,
// and the CRDs of the machines. Thus the tests show what the fake API of the
// other tests cannot show: the schema of the CRDs accepts the writes of the
// controller, the API server checks the versions and the UIDs of the writes,
// and the controller finds the machines in the objects that the CRDs serve.
//
// The clock of the reconciler is the clock of the test, but a subtest that runs
// the manager uses the real clock. The subtests run one after the other on the
// Node nodeName, and each subtest deletes its objects.

// The writers put the times of the evidence into the object. These times are
// 30s before the controller sees the evidence, so the tests show that the
// deadlines start at the observation.
const writtenBefore = 30 * time.Second

var (
	instanceKind = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha2", Kind: "Instance"}
	mhcKind      = schema.GroupVersionKind{Group: capiGroup, Version: "v1beta2", Kind: "MachineHealthCheck"}
)

func TestEvictionControllerEnvtest(t *testing.T) {
	env := testenv.Start(t, "crds/instance.yaml", "crds/mcm.yaml", "images/node-controller/crds/machine-health-check.yaml")
	api := env.Client

	create(t, api, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: agentNamespace}})

	builtin := []v1alpha1.ProfileName{v1alpha1.ProfileCritical, v1alpha1.ProfileNormal, v1alpha1.ProfileSlow}
	for _, name := range builtin {
		create(t, api, builtinProfile(t, name))
	}

	critical, _ := timingsOf(t, api, v1alpha1.ProfileCritical)

	t.Run("attested node gets the taint after the watchdog of its profile", func(t *testing.T) {
		for _, name := range builtin {
			after, _ := timingsOf(t, api, name)

			t.Run(string(name), func(t *testing.T) {
				a := newAPIHarness(t, api)
				a.attestedIncident(name, a.createNode())

				a.reconcileAt(observedAt)
				a.reconcileAt(observedAt.Add(after - time.Millisecond))

				if got := a.incident().Status.Phase; got != v1alpha1.PhaseSuspected || a.tainted() {
					t.Errorf("phase is %q, tainted %t before %s, want %q without the taint", got, a.tainted(), after, v1alpha1.PhaseSuspected)
				}

				a.reconcileAt(observedAt.Add(after))

				// The API server adds the taint not-ready to a new Node.
				taints := a.node().Spec.Taints
				if !slices.ContainsFunc(taints, func(taint corev1.Taint) bool {
					return taint.Key == corev1.TaintNodeOutOfService && taint.Value == outOfServiceValue &&
						taint.Effect == corev1.TaintEffectNoExecute
				}) {
					t.Errorf("taints are %+v at %s, want node.kubernetes.io/out-of-service=nodeshutdown:NoExecute", taints, after)
				}

				// The Node has no pods, no volumes and no machine.
				if got := a.incident().Status.Phase; got != v1alpha1.PhaseEvicted {
					t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
				}

				events := a.events()
				if countEvents(events, common.ReasonOutOfServiceTaintSet) != 2 || countEvents(events, "from Evicting to Evicted") != 2 {
					t.Errorf("events are %q, want the taint and the phase Evicted on the incident and on the node", events)
				}

				if got := testutil.ToFloat64(phaseGauge.WithLabelValues(nodeName, string(v1alpha1.PhaseEvicted), "true")); got != 1 {
					t.Errorf("d8_fencing_failed_node_state_phase{phase=Evicted} is %v, want 1", got)
				}
			})
		}
	})

	t.Run("heartbeat keeps the incident in FallbackAlive for 2 heartbeat intervals and the API timeout", func(t *testing.T) {
		for name, tc := range map[string]struct {
			profile v1alpha1.ProfileName
			// ttl is the heartbeat TTL of an unarmed node. An armed node
			// has the TTL of its profile.
			ttl time.Duration
		}{
			"armed critical":              {profile: v1alpha1.ProfileCritical},
			"unarmed with an attestation": {ttl: 20 * time.Second},
		} {
			t.Run(name, func(t *testing.T) {
				a := newAPIHarness(t, api)

				ttl := tc.ttl
				if tc.profile != "" {
					_, ttl = timingsOf(t, api, tc.profile)
				}

				incident := a.failedState(tc.profile, a.createNode())
				incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: microTime(observedAt.Add(-writtenBefore))}
				if tc.profile == "" {
					attest(incident, observedAt.Add(-writtenBefore))
				}
				a.create(incident)

				if res := a.reconcileAt(observedAt); res.RequeueAfter != ttl {
					t.Errorf("requeued after %s, want %s", res.RequeueAfter, ttl)
				}

				steps := []struct {
					at   time.Duration
					want v1alpha1.FencingFailedNodeStatePhase
				}{
					{at: ttl - time.Millisecond, want: v1alpha1.PhaseFallbackAlive},
					{at: ttl, want: v1alpha1.PhaseSuspected},
				}

				// The controller never proves the failure of an unarmed
				// node. An armed node gets the taint after the silent delay.
				if tc.profile == "" {
					steps = append(steps, struct {
						at   time.Duration
						want v1alpha1.FencingFailedNodeStatePhase
					}{at: time.Hour, want: v1alpha1.PhaseSuspected})
				}

				for _, step := range steps {
					a.reconcileAt(observedAt.Add(step.at))

					if got := a.incident().Status.Phase; got != step.want {
						t.Errorf("phase is %q %s after the heartbeat, want %q", got, step.at, step.want)
					}
				}

				if a.tainted() {
					t.Errorf("node has the taint")
				}
			})
		}
	})

	t.Run("write of Proven fails when the node wrote a heartbeat after the read", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())
		a.reconcileAt(observedAt)

		proof := observedAt.Add(critical)
		heartbeatFirst := true

		a.reconciler.client = interceptor.NewClient(api, interceptor.Funcs{
			SubResourcePatch: func(
				ctx context.Context,
				c client.Client,
				subResource string,
				obj client.Object,
				patch client.Patch,
				opts ...client.SubResourcePatchOption,
			) error {
				if _, ok := obj.(*v1alpha1.FencingFailedNodeState); ok && heartbeatFirst {
					heartbeatFirst = false

					a.writeHeartbeat(proof)
				}

				return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
			},
		})

		if res := a.reconcileAt(proof); heartbeatFirst || res.RequeueAfter <= 0 || res.RequeueAfter >= recheckAfter {
			t.Fatalf("reconcile requeued after %s, want a requeue sooner than %s after the conflict", res.RequeueAfter, recheckAfter)
		}

		// The silent delay after the heartbeat is longer than critical.
		a.reconcileAt(proof)
		a.reconcileAt(proof.Add(critical))

		if got := a.incident().Status.Phase; got == v1alpha1.PhaseProven || a.tainted() {
			t.Errorf("phase is %q, tainted %t after the heartbeat, want no proof and no taint", got, a.tainted())
		}
	})

	t.Run("heartbeat on the version before Proven fails", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())
		a.reconcileAt(observedAt)

		read := a.incident()

		a.reconcileAt(observedAt.Add(critical))

		read.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: microTime(observedAt.Add(time.Second))}

		if err := api.Status().Update(t.Context(), read); !apierrors.IsConflict(err) {
			t.Errorf("heartbeat on the old version returned %v, want a conflict", err)
		}
	})

	t.Run("heartbeat after the attestation stops the taint", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())
		a.reconcileAt(observedAt)

		// The silent delay after the heartbeat is longer than critical.
		a.writeHeartbeat(observedAt.Add(time.Second))
		a.reconcileAt(observedAt.Add(time.Second))
		a.reconcileAt(observedAt.Add(time.Second + critical))

		if a.tainted() {
			t.Errorf("node has the taint after a heartbeat")
		}

		if got := testutil.ToFloat64(aliveAfterAttestationGauge.WithLabelValues(nodeName)); got != 1 {
			t.Errorf("d8_fencing_alive_after_attestation is %v, want 1", got)
		}
	})

	t.Run("taint waits while maintenance excludes the node", func(t *testing.T) {
		exclusions := map[string]struct{ set, clear func(*corev1.Node) }{}

		for _, key := range maintenanceAnnotations {
			exclusions["annotation "+key] = struct{ set, clear func(*corev1.Node) }{
				set:   func(n *corev1.Node) { n.Annotations = map[string]string{key: ""} },
				clear: func(n *corev1.Node) { delete(n.Annotations, key) },
			}
		}

		for name, exclusion := range exclusions {
			t.Run(name, func(t *testing.T) {
				a := newAPIHarness(t, api)
				a.attestedIncident(v1alpha1.ProfileCritical, a.createNode(exclusion.set))
				a.reconcileAt(observedAt)
				a.reconcileAt(observedAt.Add(critical))

				if got := a.incident().Status.Phase; got != v1alpha1.PhaseProven || a.tainted() {
					t.Errorf("phase is %q, tainted %t, want %q without the taint", got, a.tainted(), v1alpha1.PhaseProven)
				}

				a.updateNode(exclusion.clear)
				a.reconcileAt(observedAt.Add(critical + recheckAfter))

				if !a.tainted() {
					t.Errorf("node has no taint after the end of the exclusion")
				}
			})
		}
	})

	t.Run("proven failure waits for the armed agent", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode(func(n *corev1.Node) {
			n.Status.Conditions = []corev1.NodeCondition{{
				Type: fencingConditionType, Status: corev1.ConditionFalse, Reason: "WaitingForControlPlane",
			}}
		}))
		a.reconcileAt(observedAt)
		a.reconcileAt(observedAt.Add(critical))

		if got := a.incident().Status.Phase; got != v1alpha1.PhaseSuspected || a.tainted() {
			t.Errorf("phase is %q, tainted %t, want %q without the taint", got, a.tainted(), v1alpha1.PhaseSuspected)
		}

		a.updateNode(func(n *corev1.Node) { n.Status.Conditions = []corev1.NodeCondition{armedCondition()} })
		a.reconcileAt(observedAt.Add(critical + recheckAfter))

		if !a.tainted() {
			t.Errorf("node has no taint after the agent armed")
		}
	})

	t.Run("incident gets its decision while the pods of another node are deleted", func(t *testing.T) {
		a := newAPIHarness(t, api)

		// The other Node is in eviction. The deletion of its pod does
		// not end until the end of the test.
		other := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-4"},
			Spec:       corev1.NodeSpec{Taints: []corev1.Taint{outOfServiceTaint}},
		}
		a.create(other)

		evicting := a.failedState(v1alpha1.ProfileCritical, other)
		evicting.Name = other.Name
		evicting.OwnerReferences[0].Name = other.Name
		evicting.Status.Phase = v1alpha1.PhaseEvicting

		pod := running(podOn("default", "web"))
		pod.Spec.NodeName = other.Name
		a.create(evicting, pod)

		deleting := make(chan struct{})

		var once sync.Once

		a.reconciler.client = interceptor.NewClient(api, interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if pod, ok := obj.(*corev1.Pod); ok && pod.Spec.NodeName == other.Name {
					once.Do(func() { close(deleting) })
					<-ctx.Done()

					return ctx.Err()
				}

				return c.Delete(ctx, obj, opts...)
			},
		})

		a.start(env.Config)

		select {
		case <-deleting:
		case <-time.After(10 * time.Second):
			t.Fatalf("controller did not start the deletion of the pod of the other node")
		}

		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())

		// The decision comes at the deadline of the profile, with a margin
		// for the test.
		wait := critical + 2*time.Second
		for deadline := time.Now().Add(wait); a.incident().Status.Phase != v1alpha1.PhaseEvicted; time.Sleep(100 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("phase is %q after %s, want %q", a.incident().Status.Phase, wait, v1alpha1.PhaseEvicted)
			}
		}

		if !a.tainted() {
			t.Errorf("node has no taint in the phase %q", v1alpha1.PhaseEvicted)
		}
	})

	t.Run("node that came back loses the taint and the object", func(t *testing.T) {
		a := newAPIHarness(t, api)
		node := a.createNode()
		a.attestedIncident(v1alpha1.ProfileCritical, node)
		a.reconcileAt(observedAt)
		a.reconcileAt(observedAt.Add(critical))

		if !a.tainted() {
			t.Fatalf("node has no taint before the return")
		}

		node = a.node()
		node.Status = returnedNode().Status

		if err := api.Status().Update(t.Context(), node); err != nil {
			t.Fatalf("update status of the node: %v", err)
		}

		a.reconcileAt(observedAt.Add(time.Minute))

		if a.tainted() || a.exists(&v1alpha1.FencingFailedNodeState{}, "", nodeName) {
			t.Errorf("tainted %t, object exists %t, want the taint and the object gone",
				a.tainted(), a.exists(&v1alpha1.FencingFailedNodeState{}, "", nodeName))
		}

		if got := countEvents(a.events(), common.ReasonOutOfServiceTaintRemoved); got != 2 {
			t.Errorf("controller sent %d events about the removed taint, want one on the incident and one on the node", got)
		}
	})

	t.Run("incident of a recreated node is ignored", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())

		if err := api.Delete(t.Context(), a.node()); err != nil {
			t.Fatalf("delete node: %v", err)
		}

		a.createNode()

		a.reconcileAt(observedAt)
		a.reconcileAt(observedAt.Add(time.Hour))

		if got := a.incident(); got.Status.Phase != "" || a.tainted() {
			t.Errorf("phase is %q, tainted %t, want no phase and no taint", got.Status.Phase, a.tainted())
		}
	})

	t.Run("incident without its profile reports the configuration error", func(t *testing.T) {
		a := newAPIHarness(t, api)

		if err := api.Delete(t.Context(), builtinProfile(t, v1alpha1.ProfileSlow)); err != nil {
			t.Fatalf("delete profile: %v", err)
		}

		t.Cleanup(func() {
			if err := api.Create(context.Background(), builtinProfile(t, v1alpha1.ProfileSlow)); err != nil && !apierrors.IsAlreadyExists(err) {
				t.Errorf("create profile again: %v", err)
			}
		})

		a.attestedIncident(v1alpha1.ProfileSlow, a.createNode())

		if err := a.reconcileErrAt(observedAt); err == nil {
			t.Errorf("reconcile without the profile returned no error, want a retry")
		}

		assertProfileCondition(t, a.incident(), metav1.ConditionTrue, common.ReasonProfileUnavailable)

		create(t, api, builtinProfile(t, v1alpha1.ProfileSlow))
		a.reconcileAt(observedAt.Add(time.Second))

		got := a.incident()
		assertProfileCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)

		if got.Status.Phase != v1alpha1.PhaseSuspected {
			t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseSuspected)
		}
	})

	t.Run("pods of the dead node are Failed and deleted, the eviction waits for the volumes", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())

		daemon := running(podOn("kube-system", "cilium-abcde", toleration("", corev1.TolerationOpExists)))
		daemon.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1", Kind: "DaemonSet", Name: "cilium", UID: "d5", Controller: &isController,
		}}

		mirror := running(podOn("kube-system", "kube-apiserver-"+nodeName, toleration("", corev1.TolerationOpExists)))
		mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "hash"}

		database := kept(running(podOn("default", "db-0")))
		database.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db", UID: "s1", Controller: &isController,
		}}

		// The finalizer test/keep keeps a deleted pod in the API, so the test
		// can read its status.
		a.create(daemon, mirror, database,
			running(podOn("default", "web")),
			kept(running(podOn("default", "tolerates-all", toleration("", corev1.TolerationOpExists)))),
			kept(running(podOn("kube-system", "tolerates-out-of-service", toleration(corev1.TaintNodeOutOfService, corev1.TolerationOpExists)))),
			volumeAttachment("csi-own", nodeName), volumeAttachment("csi-other", "worker-1"),
		)

		a.reconcileAt(observedAt)

		start := observedAt.Add(critical)
		a.reconcileAt(start)

		if !a.tainted() {
			t.Errorf("node has no taint")
		}

		if a.exists(&corev1.Pod{}, "default", "web") {
			t.Errorf("pod default/web without finalizers is not deleted")
		}

		for _, name := range []string{"default/db-0", "default/tolerates-all", "kube-system/tolerates-out-of-service"} {
			pod := a.pod(name)
			target := disruptionTarget(pod)

			if pod.Status.Phase != corev1.PodFailed || target == nil || target.Status != corev1.ConditionTrue ||
				target.Reason != disruptionReason {
				t.Errorf("pod %s has phase %q and DisruptionTarget %+v, want Failed with DisruptionTarget", name, pod.Status.Phase, target)
			}

			if pod.DeletionTimestamp == nil || pod.DeletionGracePeriodSeconds == nil || *pod.DeletionGracePeriodSeconds != 0 {
				t.Errorf("pod %s has deletion %v with grace %v, want a deletion with no grace period",
					name, pod.DeletionTimestamp, pod.DeletionGracePeriodSeconds)
			}

			if len(pod.Finalizers) != 1 {
				t.Errorf("pod %s has finalizers %v, want the finalizer of the test", name, pod.Finalizers)
			}
		}

		for _, name := range []string{"kube-system/cilium-abcde", "kube-system/kube-apiserver-" + nodeName} {
			if pod := a.pod(name); pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil || disruptionTarget(pod) != nil {
				t.Errorf("pod %s has phase %q and deletion %v, want a running pod", name, pod.Status.Phase, pod.DeletionTimestamp)
			}
		}

		a.reconcileAt(start.Add(recheckAfter))
		a.reconcileAt(start.Add(evictionTimeout - time.Millisecond))

		if got := a.incident(); got.Status.Phase != v1alpha1.PhaseEvicting ||
			meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing) != nil {
			t.Errorf("phase is %q with conditions %+v before the timeout, want %q without Progressing",
				got.Status.Phase, got.Status.Conditions, v1alpha1.PhaseEvicting)
		}

		a.reconcileAt(start.Add(evictionTimeout))

		got := a.incident()
		assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonVolumesAttached)

		if message := meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing).Message; !strings.Contains(message, "csi-own") ||
			strings.Contains(message, "csi-other") {
			t.Errorf("condition Progressing has the message %q, want the attachment of the node only", message)
		}

		// The attach-detach controller detaches the volume.
		if err := api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
			t.Fatalf("delete volumeattachment: %v", err)
		}

		a.reconcileAt(start.Add(evictionTimeout + recheckAfter))

		got = a.incident()
		if got.Status.Phase != v1alpha1.PhaseEvicted || meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing) != nil {
			t.Errorf("phase is %q with conditions %+v, want %q without Progressing", got.Status.Phase, got.Status.Conditions, v1alpha1.PhaseEvicted)
		}
	})

	t.Run("node of a cloud CAPI machine is deleted when the MachineHealthCheck replaces it", func(t *testing.T) {
		for name, tc := range map[string]struct {
			healthy int64
			deleted bool
		}{
			"1 of 4 unhealthy": {healthy: 3, deleted: true},
			"3 of 4 unhealthy": {healthy: 1},
		} {
			t.Run(name, func(t *testing.T) {
				a := newAPIHarness(t, api)
				a.create(apiHealthCheck("dvp", 4, tc.healthy))
				a.attestedIncident(v1alpha1.ProfileCritical, a.createNode(capiNode("dvp")))

				a.reconcileAt(observedAt)

				start := observedAt.Add(critical)
				a.reconcileAt(start)

				if got := a.exists(&corev1.Node{}, "", nodeName); got == tc.deleted {
					t.Fatalf("node exists: %t, want %t", got, !tc.deleted)
				}

				if tc.deleted {
					return
				}

				a.reconcileAt(start.Add(evictionTimeout))

				got := a.incident()
				if got.Status.Phase != v1alpha1.PhaseEvicting || !a.tainted() {
					t.Errorf("phase is %q, tainted %t, want %q with the taint", got.Status.Phase, a.tainted(), v1alpha1.PhaseEvicting)
				}

				assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonTooManyUnhealthyMachines)
			})
		}
	})

	t.Run("machine of an MCM node is deleted", func(t *testing.T) {
		a := newAPIHarness(t, api)
		a.create(instance("instance-1", mcmGroup+"/v1alpha1", nodeName), machine(nodeName), machine("other"))
		a.attestedIncident(v1alpha1.ProfileCritical, a.createNode())

		a.reconcileAt(observedAt)
		a.reconcileAt(observedAt.Add(critical))

		if got := a.incident().Status.Phase; got != v1alpha1.PhaseEvicted {
			t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
		}

		if a.exists(newMachine(), agentNamespace, nodeName) || !a.exists(newMachine(), agentNamespace, "other") {
			t.Errorf("machine %s exists %t, machine other exists %t, want only the machine of the node deleted",
				nodeName, a.exists(newMachine(), agentNamespace, nodeName), a.exists(newMachine(), agentNamespace, "other"))
		}
	})

	t.Run("node that is not replaced keeps only the taint", func(t *testing.T) {
		label := func(key string) func(*corev1.Node) {
			return func(n *corev1.Node) {
				capiNode("dvp")(n)
				n.Labels = map[string]string{key: ""}
			}
		}

		for name, change := range map[string]func(*corev1.Node){
			"master":              label("node-role.kubernetes.io/control-plane"),
			"etcd arbiter":        label("node.deckhouse.io/etcd-arbiter"),
			"static CAPI cluster": capiNode(staticClusterName),
			// Static, CloudPermanent and CloudStatic nodes have no machine.
			"node without a machine": func(*corev1.Node) {},
		} {
			t.Run(name, func(t *testing.T) {
				a := newAPIHarness(t, api)
				a.create(apiHealthCheck("dvp", 4, 4), apiHealthCheck(staticClusterName, 4, 4))
				a.attestedIncident(v1alpha1.ProfileCritical, a.createNode(change))

				a.reconcileAt(observedAt)
				a.reconcileAt(observedAt.Add(critical))

				if got := a.incident().Status.Phase; got != v1alpha1.PhaseEvicted || !a.tainted() {
					t.Errorf("phase is %q, tainted %t, want %q with the taint", got, a.tainted(), v1alpha1.PhaseEvicted)
				}
			})
		}
	})
}

// apiHarness runs the reconciler with the real adapters against the API server
// of the test.
type apiHarness struct {
	t          *testing.T
	api        client.WithWatch
	reconciler *Reconciler
	recorder   *record.FakeRecorder
}

func newAPIHarness(t *testing.T, api client.WithWatch) *apiHarness {
	t.Helper()

	resetMetrics()

	a := &apiHarness{t: t, api: api, recorder: record.NewFakeRecorder(eventBuffer)}
	a.reconciler = New(api, api, agentNamespace,
		noderef.NewValidator(),
		profile.NewResolver(fencingstate.NewProfiles(api)),
		a.recorder,
	)

	t.Cleanup(a.sweep)

	return a
}

// start runs the reconciler in a manager until the end of the test. The clock
// of the reconciler is the real clock.
func (a *apiHarness) start(cfg *rest.Config) {
	a.t.Helper()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:     a.api.Scheme(),
		Metrics:    metricsserver.Options{BindAddress: "0"},
		Controller: config.Controller{SkipNameValidation: ptr.To(true)},
	})
	if err != nil {
		a.t.Fatalf("create manager: %v", err)
	}

	if err := a.reconciler.SetupWithManager(mgr); err != nil {
		a.t.Fatalf("set up the controller: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error)

	go func() { stopped <- mgr.Start(ctx) }()

	a.t.Cleanup(func() {
		cancel()

		if err := <-stopped; err != nil {
			a.t.Errorf("manager stopped with %v", err)
		}
	})
}

func (a *apiHarness) reconcileErrAt(at time.Time) error {
	a.t.Helper()

	a.reconciler.now = func() time.Time { return at }

	_, err := a.reconciler.Reconcile(a.t.Context(), request(nodeName))

	return err
}

func (a *apiHarness) reconcileAt(at time.Time) ctrl.Result {
	a.t.Helper()

	a.reconciler.now = func() time.Time { return at }

	res, err := a.reconciler.Reconcile(a.t.Context(), request(nodeName))
	if err != nil {
		a.t.Fatalf("reconcile at %s: %v", at, err)
	}

	return res
}

func (a *apiHarness) create(objects ...client.Object) {
	a.t.Helper()

	create(a.t, a.api, objects...)
}

// createNode creates the Node nodeName and returns it. The agent of the Node
// wrote that the watchdog is armed.
func (a *apiHarness) createNode(changes ...func(*corev1.Node)) *corev1.Node {
	a.t.Helper()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{armedCondition()}},
	}
	for _, change := range changes {
		change(node)
	}

	a.create(node)

	return node
}

// failedState returns the object of the incident of the Node, as a writer
// creates it. An empty profile is an unarmed node.
func (a *apiHarness) failedState(name v1alpha1.ProfileName, node *corev1.Node) *v1alpha1.FencingFailedNodeState {
	incident := failedState()
	incident.UID = ""
	incident.Generation = 0
	incident.OwnerReferences[0].UID = node.UID

	if name == "" {
		incident.Spec.Armed = false
		incident.Spec.ProfileRef = nil
	} else {
		incident.Spec.ProfileRef.Name = name
	}

	return incident
}

// attestedIncident creates the object of the incident of the Node with the
// attestation of a witness.
func (a *apiHarness) attestedIncident(name v1alpha1.ProfileName, node *corev1.Node) {
	a.t.Helper()

	incident := a.failedState(name, node)
	attest(incident, observedAt.Add(-writtenBefore))
	a.create(incident)
}

func (a *apiHarness) incident() *v1alpha1.FencingFailedNodeState {
	a.t.Helper()

	var incident v1alpha1.FencingFailedNodeState
	if err := a.api.Get(a.t.Context(), types.NamespacedName{Name: nodeName}, &incident); err != nil {
		a.t.Fatalf("get fencingfailednodestate: %v", err)
	}

	return &incident
}

// writeHeartbeat writes a heartbeat to the object, as the failed node does.
func (a *apiHarness) writeHeartbeat(at time.Time) {
	a.t.Helper()

	incident := a.incident()
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: microTime(at)}

	if err := a.api.Status().Update(a.t.Context(), incident); err != nil {
		a.t.Fatalf("write heartbeat: %v", err)
	}
}

// updateNode changes the Node and its status.
func (a *apiHarness) updateNode(change func(*corev1.Node)) {
	a.t.Helper()

	node := a.node()
	change(node)

	if err := a.api.Update(a.t.Context(), node); err != nil {
		a.t.Fatalf("update node: %v", err)
	}

	// The update keeps the old status, so the change goes to the status too.
	change(node)

	if err := a.api.Status().Update(a.t.Context(), node); err != nil {
		a.t.Fatalf("update status of the node: %v", err)
	}
}

func (a *apiHarness) node() *corev1.Node {
	a.t.Helper()

	var node corev1.Node
	if err := a.api.Get(a.t.Context(), types.NamespacedName{Name: nodeName}, &node); err != nil {
		a.t.Fatalf("get node: %v", err)
	}

	return &node
}

func (a *apiHarness) tainted() bool {
	a.t.Helper()

	for _, taint := range a.node().Spec.Taints {
		if taint.MatchTaint(&outOfServiceTaint) {
			return true
		}
	}

	return false
}

// pod returns the pod with the name namespace/name.
func (a *apiHarness) pod(name string) *corev1.Pod {
	a.t.Helper()

	namespace, name, _ := strings.Cut(name, "/")

	var pod corev1.Pod
	if err := a.api.Get(a.t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		a.t.Fatalf("get pod: %v", err)
	}

	return &pod
}

func (a *apiHarness) exists(obj client.Object, namespace, name string) bool {
	a.t.Helper()

	err := a.api.Get(a.t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		a.t.Fatalf("get %s/%s: %v", namespace, name, err)
	}

	return err == nil
}

// events drains what the reconciler published, as "Type Reason message" lines.
func (a *apiHarness) events() []string {
	var published []string

	for {
		select {
		case event := <-a.recorder.Events:
			published = append(published, event)
		default:
			return published
		}
	}
}

// sweep deletes the objects of a subtest. The test context is canceled before
// the cleanup, so the cleanup uses its own context.
func (a *apiHarness) sweep() {
	ctx := context.Background()

	var pods corev1.PodList
	if err := a.api.List(ctx, &pods); err != nil {
		a.t.Errorf("list pods: %v", err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if len(pod.Finalizers) > 0 {
			pod.Finalizers = nil
			if err := a.api.Update(ctx, pod); err != nil && !apierrors.IsNotFound(err) {
				a.t.Errorf("remove finalizers of pod %s/%s: %v", pod.Namespace, pod.Name, err)
			}
		}

		if err := a.api.Delete(ctx, pod, client.GracePeriodSeconds(0)); err != nil && !apierrors.IsNotFound(err) {
			a.t.Errorf("delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}

	for _, obj := range []client.Object{
		&v1alpha1.FencingFailedNodeState{},
		&corev1.Node{},
		&storagev1.VolumeAttachment{},
		kindOf(instanceKind),
	} {
		if err := a.api.DeleteAllOf(ctx, obj); err != nil {
			a.t.Errorf("delete all %T: %v", obj, err)
		}
	}

	for _, obj := range []client.Object{kindOf(mhcKind), kindOf(mcmMachine)} {
		if err := a.api.DeleteAllOf(ctx, obj, client.InNamespace(agentNamespace)); err != nil {
			a.t.Errorf("delete all %T: %v", obj, err)
		}
	}
}

// create creates the objects and then writes their status, because the API
// server drops the status of a new object. It adds the fields that the API
// server needs and the tests of the fake API do not set.
func create(t *testing.T, api client.Client, objects ...client.Object) {
	t.Helper()

	for _, obj := range objects {
		// restore puts the status back after the create. It returns false
		// when there is no status to write.
		var restore func() bool

		switch o := obj.(type) {
		case *v1alpha1.FencingFailedNodeState:
			saved := o.Status
			restore = func() bool { o.Status = saved; return !reflect.ValueOf(saved).IsZero() }
		case *corev1.Node:
			saved := o.Status
			restore = func() bool { o.Status = saved; return !reflect.ValueOf(saved).IsZero() }
		case *corev1.Pod:
			o.Spec.Containers = []corev1.Container{{Name: "app", Image: "app"}}
			saved := o.Status
			restore = func() bool { o.Status = saved; return !reflect.ValueOf(saved).IsZero() }
		case *storagev1.VolumeAttachment:
			pv := "pv-" + o.Name
			o.Spec.Source.PersistentVolumeName = &pv
		case *unstructured.Unstructured:
			saved, ok := o.Object["status"]
			restore = func() bool { o.Object["status"] = saved; return ok }
		}

		obj.SetUID("")

		if err := api.Create(t.Context(), obj); err != nil {
			t.Fatalf("create %T %s: %v", obj, obj.GetName(), err)
		}

		if restore == nil || !restore() {
			continue
		}

		if err := api.Status().Update(t.Context(), obj); err != nil {
			t.Fatalf("write status of %T %s: %v", obj, obj.GetName(), err)
		}
	}
}

// timingsOf returns the timings of the profile that the test reads from the
// API: the taint of an attested node comes watchdog.timeout + watchdog.margin
// after the attestation, and a heartbeat keeps FallbackAlive for
// 2 * heartbeat.interval + heartbeat.apiTimeout.
func timingsOf(t *testing.T, api client.Client, name v1alpha1.ProfileName) (taintAfter, heartbeatTTL time.Duration) {
	t.Helper()

	var profile v1alpha1.FencingSLAProfile
	if err := api.Get(t.Context(), types.NamespacedName{Name: name.ObjectName()}, &profile); err != nil {
		t.Fatalf("get profile %s: %v", name, err)
	}

	spec := profile.Spec

	return spec.Watchdog.Timeout.Duration + spec.Watchdog.Margin.Duration,
		2*spec.Heartbeat.Interval.Duration + spec.Heartbeat.APITimeout.Duration
}

// builtinProfile reads the built-in profile from the templates of the module.
func builtinProfile(t *testing.T, name v1alpha1.ProfileName) client.Object {
	t.Helper()

	return testenv.Manifests(t, "templates/fencing-agent/sla-profiles/"+name.ObjectName()+".yaml")[0]
}

// apiHealthCheck is a MachineHealthCheck with the fields that its CRD needs.
func apiHealthCheck(cluster string, expected, healthy int64) *unstructured.Unstructured {
	check := healthCheck(cluster, expected, healthy)
	check.Object["spec"].(map[string]any)["selector"] = map[string]any{}

	return check
}

func kindOf(kind schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(kind)

	return obj
}
