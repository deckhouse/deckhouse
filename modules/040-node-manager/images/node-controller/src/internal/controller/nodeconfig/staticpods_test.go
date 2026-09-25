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

package nodeconfig

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// podManifest is the smallest document ValidateStaticPodManifest accepts. The
// pod's name is deliberately not the object's: the two are unrelated, and a
// helper that made them equal would hide every place that still assumes they are.
func podManifest(pod string) string {
	return "apiVersion: v1\nkind: Pod\nmetadata:\n  name: " + pod + "\n  namespace: d8-system\n" +
		"spec:\n  hostNetwork: true\n  containers:\n  - name: main\n    image: deckhouse.local/images:" + pod + "\n"
}

// nspr builds an object whose manifest names a pod after the object itself.
func nspr(name string, spec deckhousev1alpha1.NodeStaticPodRequestSpec) deckhousev1alpha1.NodeStaticPodRequest {
	if spec.Manifest == "" {
		spec.Manifest = podManifest(name)
	}
	return deckhousev1alpha1.NodeStaticPodRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
}

// nsprCreated is the same with a creation time, which is what the pod contest is
// settled on.
func nsprCreated(name string, created metav1.Time, spec deckhousev1alpha1.NodeStaticPodRequestSpec) deckhousev1alpha1.NodeStaticPodRequest {
	object := nspr(name, spec)
	object.CreationTimestamp = created
	return object
}

// staticPodsOf runs a listing through the ordering and the checks a pass settles
// once, the way renderSpec receives them.
func staticPodsOf(nsprs []deckhousev1alpha1.NodeStaticPodRequest, ngName string) []internalv1alpha1.StaticPod {
	ordered := orderedNSPRs(nsprs)
	return nodeStaticPods(ordered, rejectedNSPRs(ordered), ngName)
}

func TestNodeStaticPodRequests(t *testing.T) {
	tests := []struct {
		name   string
		nsprs  []deckhousev1alpha1.NodeStaticPodRequest
		ngName string
		want   []internalv1alpha1.StaticPod
	}{
		{
			name: "the manifest passes through byte for byte, matched by nodeGroupSelector",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"worker"}},
				}),
			},
			ngName: "worker",
			want: []internalv1alpha1.StaticPod{{
				Name:     "registry-agent",
				Manifest: podManifest("registry-agent"),
			}},
		},
		{
			// The entry's name is the object's — the file name on the node — and the
			// pod inside it is called something else entirely. Nothing renames
			// either to match the other.
			name: "the file name is the object's, whatever the pod is called",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("agent-request", deckhousev1alpha1.NodeStaticPodRequestSpec{
					Manifest: podManifest("registry-agent"),
				}),
			},
			ngName: "worker",
			want: []internalv1alpha1.StaticPod{{
				Name:     "agent-request",
				Manifest: podManifest("registry-agent"),
			}},
		},
		{
			name: "not matched when the selector names another group",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"storage"}},
				}),
			},
			ngName: "worker",
		},
		{
			name: "an empty selector selects every group",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{}),
			},
			ngName: "worker",
			want:   []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
		},
		{
			// The object name becomes spec.staticPods[].name, which is a DNS label;
			// a CR name is a DNS subdomain. Rendering this one costs the node its
			// whole NodeConfig, so it never leaves the controller.
			name: "a name the NodeConfig field would not take contributes nothing",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("registry-agent.v2", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("registry-agent")}),
			},
			ngName: "worker",
		},
		{
			name: "a reserved control-plane name contributes nothing",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("etcd", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("etcd")}),
			},
			ngName: "worker",
		},
		{
			// A bashible step writes this manifest itself, on every node of every
			// mutable group; the object would replace the node's own API proxy.
			name: "a name a bashible step writes itself contributes nothing",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("kubernetes-api-proxy", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
			},
			ngName: "worker",
		},
		{
			name: "a manifest that is not a Pod contributes nothing",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: "apiVersion: apps/v1\nkind: Deployment\n"}),
			},
			ngName: "worker",
		},
		{
			// Both entries would go into one NodeConfig and the node would keep
			// whichever comes first, not the older object.
			name: "two objects on one pod: only the older one reaches the node",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nsprCreated("newer", metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
					deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("registry-agent")}),
				nsprCreated("older", metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("registry-agent")}),
			},
			ngName: "worker",
			want:   []internalv1alpha1.StaticPod{{Name: "older", Manifest: podManifest("registry-agent")}},
		},
		{
			// One pod name in two namespaces is two pods, and kubelet runs both.
			name: "the same pod name in two namespaces is not a conflict",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("system", deckhousev1alpha1.NodeStaticPodRequestSpec{
					Manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n  namespace: d8-system\n",
				}),
				nspr("kube", deckhousev1alpha1.NodeStaticPodRequestSpec{
					Manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n  namespace: kube-system\n",
				}),
			},
			ngName: "worker",
			want: []internalv1alpha1.StaticPod{
				{Name: "kube", Manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n  namespace: kube-system\n"},
				{Name: "system", Manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n  namespace: d8-system\n"},
			},
		},
		{
			// One bad object is one bad object: the node still gets the others.
			name: "a bad object does not take the good ones with it",
			nsprs: []deckhousev1alpha1.NodeStaticPodRequest{
				nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: "apiVersion: apps/v1\nkind: Deployment\n"}),
				nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{}),
			},
			ngName: "worker",
			want:   []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods := staticPodsOf(tt.nsprs, tt.ngName)
			if !reflect.DeepEqual(pods, tt.want) {
				t.Fatalf("static pods = %#v, want %#v", pods, tt.want)
			}
		})
	}
}

// spec.staticPods is an array, so a listing-order dependency was a spec diff — a
// generation bump and a rollout slot per node — for no change at all.
func TestNodeStaticPodRequestsOrderDoesNotFollowTheListing(t *testing.T) {
	zebra := nspr("zebra", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	apple := nspr("apple", deckhousev1alpha1.NodeStaticPodRequestSpec{})

	oneWay := staticPodsOf([]deckhousev1alpha1.NodeStaticPodRequest{zebra, apple}, "worker")
	otherWay := staticPodsOf([]deckhousev1alpha1.NodeStaticPodRequest{apple, zebra}, "worker")

	require.Equal(t, oneWay, otherWay, "the static pods depend on the listing order")
	require.Equal(t, []string{"apple", "zebra"}, []string{oneWay[0].Name, oneWay[1].Name},
		"the rendered array is sorted by name, whatever the listing said")

	// The contest runs oldest first, the array is rendered by name: creating an
	// object must not reshuffle the entries of every node it does not even reach.
	byName := staticPodsOf([]deckhousev1alpha1.NodeStaticPodRequest{
		nsprCreated("zebra", metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), deckhousev1alpha1.NodeStaticPodRequestSpec{}),
		nsprCreated("apple", metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)), deckhousev1alpha1.NodeStaticPodRequestSpec{}),
	}, "worker")
	require.Equal(t, []string{"apple", "zebra"}, []string{byName[0].Name, byName[1].Name},
		"the rendered array is sorted by name, not by creation order")
}

// An object this controller refused never reaches a node, so the reason has to
// be recorded here — nothing downstream can work it out.
func TestRejectedNSPRs(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	reject := func(nsprs ...deckhousev1alpha1.NodeStaticPodRequest) map[string]nsprRefusal {
		return rejectedNSPRs(orderedNSPRs(nsprs))
	}

	t.Run("nothing wrong with either of these", func(t *testing.T) {
		require.Empty(t, reject(
			nspr("alpha", deckhousev1alpha1.NodeStaticPodRequestSpec{}),
			nspr("beta", deckhousev1alpha1.NodeStaticPodRequestSpec{}),
		))
	})

	// Checked first: the API server let this name through and the NodeConfig
	// field will not, so it is the one refusal that would otherwise wedge the
	// whole node config.
	t.Run("a name the NodeConfig field would not take is refused", func(t *testing.T) {
		rejected := reject(nspr("registry-agent.v2", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: podManifest("registry-agent"),
		}))
		require.Equal(t, reasonInvalidName, rejected["registry-agent.v2"].reason)
		require.Contains(t, rejected["registry-agent.v2"].message, "registry-agent.v2")
	})

	// A refused name claims no pod either, so a typo does not take the object
	// that spells it right down with it.
	t.Run("an object with a refused name does not hold the pod it named", func(t *testing.T) {
		rejected := reject(
			nsprCreated("agent.v2", older, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
			nsprCreated("legitimate", newer, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
		)
		require.Equal(t, reasonInvalidName, rejected["agent.v2"].reason)
		require.NotContains(t, rejected, "legitimate")
	})

	t.Run("a reserved control-plane name is refused", func(t *testing.T) {
		rejected := reject(nspr("kube-apiserver", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: podManifest("kube-apiserver"),
		}))
		require.Equal(t, reasonReservedName, rejected["kube-apiserver"].reason)
		require.Contains(t, rejected["kube-apiserver"].message, "writes itself")
	})

	// kubelet collides on the pod, so the file name alone does not protect it.
	t.Run("a reserved pod under another name is refused", func(t *testing.T) {
		rejected := reject(nspr("aaa", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: strings.Replace(podManifest("etcd"), "d8-system", "kube-system", 1),
		}))
		require.Equal(t, reasonReservedName, rejected["aaa"].reason)
		require.Contains(t, rejected["aaa"].message, "kube-system/etcd")
	})

	t.Run("a manifest with no namespace is refused with the checker's own reason", func(t *testing.T) {
		rejected := reject(nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: agent\n",
		}))
		require.Equal(t, reasonInvalidManifest, rejected["broken"].reason)
		require.Equal(t, "manifest is not a valid Pod: metadata.namespace is empty", rejected["broken"].message)
	})

	t.Run("a manifest that is not a Pod is refused with one message", func(t *testing.T) {
		rejected := reject(nspr("broken", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: agent\n  namespace: d8-system\n",
		}))
		require.Equal(t, reasonInvalidManifest, rejected["broken"].reason)
		require.Contains(t, rejected["broken"].message, "manifest is not a valid Pod")
	})

	// The name the object carries is the file name and nothing else, so the two
	// have no reason to agree — and refusing that was this check's whole job
	// before revision 3.1.
	t.Run("a pod named nothing like its object is fine", func(t *testing.T) {
		require.Empty(t, reject(nspr("agent-request", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: podManifest("registry-agent"),
		})))
	})

	t.Run("the younger of two objects on one pod loses", func(t *testing.T) {
		rejected := reject(
			nsprCreated("newer", newer, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
			nsprCreated("older", older, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
		)
		require.Equal(t, reasonConflict, rejected["newer"].reason)
		require.Contains(t, rejected["newer"].message, `"older"`)
		require.Contains(t, rejected["newer"].message, "d8-system/agent")
		require.NotContains(t, rejected, "older")
	})

	// Same instant, so the contest falls through to the name — otherwise the
	// winner would depend on the listing and the fleet would flap between two
	// documents once a minute.
	t.Run("a tie is broken by name", func(t *testing.T) {
		rejected := reject(
			nsprCreated("zebra", older, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
			nsprCreated("apple", older, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
		)
		require.Equal(t, reasonConflict, rejected["zebra"].reason)
		require.NotContains(t, rejected, "apple")
	})

	// A refused object claims nothing: the pod it named is still free for the
	// next one, or the reserved-name object would take a pod down with it.
	t.Run("a refused object does not hold the pod it named", func(t *testing.T) {
		rejected := reject(
			nsprCreated("etcd", older, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
			nsprCreated("legitimate", newer, deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: podManifest("agent")}),
		)
		require.Equal(t, reasonReservedName, rejected["etcd"].reason)
		require.NotContains(t, rejected, "legitimate")
	})

	// The object's own name is checked first: a manifest that is also broken
	// would otherwise be reported as the operator's typo rather than as the one
	// thing they may not do at all.
	t.Run("a reserved name beats a broken manifest", func(t *testing.T) {
		rejected := reject(nspr("etcd", deckhousev1alpha1.NodeStaticPodRequestSpec{
			Manifest: "apiVersion: apps/v1\nkind: Deployment\n",
		}))
		require.Equal(t, reasonReservedName, rejected["etcd"].reason)
	})
}

// The contest is cluster-wide: a younger object naming a pod an older one already
// asked for is refused in every group, even one the older object never selects.
func TestAYoungerDuplicateIsRefusedInEveryGroup(t *testing.T) {
	manifest := nspr("shared", deckhousev1alpha1.NodeStaticPodRequestSpec{}).Spec.Manifest
	nsprs := []deckhousev1alpha1.NodeStaticPodRequest{
		nsprCreated("older", metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), deckhousev1alpha1.NodeStaticPodRequestSpec{
			NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"a"}},
			Manifest:          manifest,
		}),
		nsprCreated("younger", metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)), deckhousev1alpha1.NodeStaticPodRequestSpec{
			NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"b"}},
			Manifest:          manifest,
		}),
	}
	inA := staticPodsOf(nsprs, "a")
	require.Len(t, inA, 1)
	require.Equal(t, "older", inA[0].Name)
	require.Empty(t, staticPodsOf(nsprs, "b"))
}
