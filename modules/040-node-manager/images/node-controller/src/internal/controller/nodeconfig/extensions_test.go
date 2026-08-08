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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

const (
	drbdDigest  = "sha256:5b6821d5e191ed505ece10ecfc3d17a85d7f57c360e1ace11f3d46aad6203842"
	otherDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

func ner(name string, spec deckhousev1alpha1.NodeExtensionRequestSpec) deckhousev1alpha1.NodeExtensionRequest {
	return deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
}

func nerCreated(name string, created metav1.Time, spec deckhousev1alpha1.NodeExtensionRequestSpec) deckhousev1alpha1.NodeExtensionRequest {
	n := ner(name, spec)
	n.CreationTimestamp = created
	return n
}

func nodeWith(labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: labels}}
}

func TestNodeExtensions(t *testing.T) {
	// The extension a bare name+digest request resolves to: fields pass straight
	// through and RequestedBy is the request's own name.
	drbdExtension := internalv1alpha1.Extension{
		Name:        "drbd",
		Digest:      drbdDigest,
		RequestedBy: "sds-drbd",
	}

	older := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name           string
		ners           []deckhousev1alpha1.NodeExtensionRequest
		node           *corev1.Node
		ngName         string
		wantExtensions []internalv1alpha1.Extension
		wantModules    []internalv1alpha1.KernelModule
	}{
		{
			name: "name and digest pass through, matched by nodeGroupSelector",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:            deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"worker"}},
				}),
			},
			node:           nodeWith(nil),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
		},
		{
			name: "not matched when nodeGroupSelector names another group",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:            deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"storage"}},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "matched by nodeSelector labels",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:       deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}),
			},
			node:           nodeWith(map[string]string{"storage": "drbd"}),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
		},
		{
			name: "not matched when a required label is missing",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:       deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}),
			},
			node:   nodeWith(map[string]string{"role": "worker"}),
			ngName: "worker",
		},
		{
			name: "repository and path pass through verbatim",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{
						Name:       "drbd",
						Digest:     drbdDigest,
						Repository: "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
						Path:       "sds-replicated-volume",
					},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
				AdditionalPath: "sds-replicated-volume",
				Digest:         drbdDigest,
				RequestedBy:    "sds-drbd",
			}},
		},
		{
			name: "missing digest drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd"},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "reserved sysext name drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("shadow-kubelet", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "kubelet", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "name conflict: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("drbd-new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: otherDigest},
				}),
				nerCreated("drbd-old", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "drbd",
				Digest:      drbdDigest,
				RequestedBy: "drbd-old",
			}},
		},
		{
			name: "digest conflict: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("beta", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "beta", Digest: drbdDigest},
				}),
				nerCreated("alpha", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "alpha",
				Digest:      drbdDigest,
				RequestedBy: "alpha",
			}},
		},
		{
			// The loser of the module contest keeps nothing: its sysext would be
			// merged onto a node running module parameters it did not ask for, and
			// its own status says which request took the module.
			name: "a request that lost a kernel module contributes nothing",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "beta", Digest: otherDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=enabled"}}},
				}),
				nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "alpha",
				Digest:      drbdDigest,
				RequestedBy: "zebra",
			}},
			wantModules: []internalv1alpha1.KernelModule{
				{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
			},
		},
		{
			name: "kernel modules collected and deduplicated by name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{
						{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
						{Name: "drbd_transport_tcp"},
					},
				}),
			},
			node:           nodeWith(nil),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
			wantModules: []internalv1alpha1.KernelModule{
				{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
				{Name: "drbd_transport_tcp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions, modules := nodeExtensions(tt.ners, tt.node, tt.ngName)
			if !reflect.DeepEqual(extensions, tt.wantExtensions) {
				t.Fatalf("extensions = %#v, want %#v", extensions, tt.wantExtensions)
			}
			if !reflect.DeepEqual(modules, tt.wantModules) {
				t.Fatalf("modules = %#v, want %#v", modules, tt.wantModules)
			}
		})
	}
}

// spec.extensions is an array, so the order the requests are walked in is the
// order the node is given them in. Taking that from the caller made the rendered
// spec depend on which reader listed the requests: a listing that came back
// another way round was a spec diff on every node of every immutable group — a
// generation bump each and a rollout slot each, for no change at all.
func TestNodeExtensionsOrderDoesNotFollowTheListing(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	first := nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
		Sysext:        deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest},
		KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd"}},
	})
	second := nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
		Sysext:        deckhousev1alpha1.Sysext{Name: "beta", Digest: otherDigest},
		KernelModules: []deckhousev1alpha1.KernelModule{{Name: "nvidia"}},
	})

	// The same two requests, listed both ways round.
	oneWay, oneWayModules := nodeExtensions([]deckhousev1alpha1.NodeExtensionRequest{first, second}, nodeWith(nil), "worker")
	otherWay, otherWayModules := nodeExtensions([]deckhousev1alpha1.NodeExtensionRequest{second, first}, nodeWith(nil), "worker")

	if !reflect.DeepEqual(oneWay, otherWay) {
		t.Fatalf("extensions depend on the listing order: %#v vs %#v", oneWay, otherWay)
	}
	if !reflect.DeepEqual(oneWayModules, otherWayModules) {
		t.Fatalf("kernel modules depend on the listing order: %#v vs %#v", oneWayModules, otherWayModules)
	}

	// And it is the order every decision about them is made in: oldest first,
	// whatever the listing said.
	got := []string{oneWay[0].RequestedBy, oneWay[1].RequestedBy}
	if !reflect.DeepEqual(got, []string{"zebra", "apple"}) {
		t.Fatalf("extensions = %v, want the oldest request first", got)
	}
}

func TestResolveNERConflicts(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	same := metav1.NewTime(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name string
		ners []deckhousev1alpha1.NodeExtensionRequest
		want map[string]nerConflict
	}{
		{
			name: "no conflicts",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("a", deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest}}),
				ner("b", deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "beta", Digest: otherDigest}}),
			},
			want: map[string]nerConflict{},
		},
		{
			name: "name conflict: older wins, newer loses on name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: otherDigest}}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"new": {reason: reasonConflict, winner: "old", field: "name"},
			},
		},
		{
			name: "digest conflict: older wins, newer loses on digest",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "beta", Digest: drbdDigest}}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"new": {reason: reasonConflict, winner: "old", field: "digest"},
			},
		},
		{
			name: "equal timestamps: lexicographically smaller name wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("zebra", same, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: otherDigest}}),
				nerCreated("apple", same, deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"zebra": {reason: reasonConflict, winner: "apple", field: "name"},
			},
		},
		{
			name: "reserved name always loses",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("shadow", deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "kubelet", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"shadow": {reason: reasonReservedName},
			},
		},
		{
			name: "invalid sysext does not take part",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("no-digest", deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Name: "drbd"}}),
				ner("no-name", deckhousev1alpha1.NodeExtensionRequestSpec{Sysext: deckhousev1alpha1.Sysext{Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{},
		},
		{
			// Two requests loading one module with different parameters: the node
			// can only have one of them. Deciding by the order an API listing came
			// back in made the parameters a node ran depend on the alphabet, and
			// left the request that lost saying it had resolved.
			name: "kernel module with different parameters: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "beta", Digest: otherDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=enabled"}}},
				}),
				nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}},
				}),
			},
			want: map[string]nerConflict{
				"apple": {reason: reasonModuleConflict, winner: "zebra", field: "drbd"},
			},
		},
		{
			name: "the same module with the same parameters is what both asked for",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "beta", Digest: otherDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}},
				}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:        deckhousev1alpha1.Sysext{Name: "alpha", Digest: drbdDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}},
				}),
			},
			want: map[string]nerConflict{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveNERConflicts(tt.ners)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("resolveNERConflicts = %#v, want %#v", got, tt.want)
			}
		})
	}
}
