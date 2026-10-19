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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
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

// extensionsOf runs a listing of requests through the settlement a pass makes once,
// every request asking for a system extension, the way renderSpec receives them.
func extensionsOf(ners []deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string) ([]internalv1alpha1.Extension, []internalv1alpha1.KernelModule) {
	for i := range ners {
		ners[i].Spec.Extension.Kind = deckhousev1alpha1.ExtensionKindSystemExtension
	}
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs(ners, noPlatform)
	return nodeExtensions(in, deckhousev1alpha1.ExtensionKindSystemExtension, node, ngName)
}

// requestedNames are the names of the entries a request put into a document.
func requestedNames(extensions []internalv1alpha1.Extension) []string {
	var names []string
	for _, ext := range extensions {
		if ext.RequestedBy != nil && ext.RequestedBy.Kind == internalv1alpha1.ObjectOwnerKindNodeExtensionRequest {
			names = append(names, ext.Name)
		}
	}
	return names
}

// nerOwner is the requestedBy of an entry the named NodeExtensionRequest put there.
func nerOwner(name string) *internalv1alpha1.ObjectOwner {
	return &internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindNodeExtensionRequest, Name: name}
}

func TestNodeExtensions(t *testing.T) {
	// The extension a bare name+digest request resolves to: fields pass straight
	// through and RequestedBy is the request's own name.
	drbdExtension := internalv1alpha1.Extension{
		Name:        "drbd",
		Digest:      drbdDigest,
		RequestedBy: nerOwner("sds-drbd"),
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
					Extension:         deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest},
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
					Extension:         deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest},
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
					Extension:    deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest},
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
					Extension:    deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest},
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
					Extension: deckhousev1alpha1.NodeExtension{
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
				Name:        "drbd",
				Digest:      drbdDigest,
				Repository:  "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
				Path:        "sds-replicated-volume",
				RequestedBy: nerOwner("sds-drbd"),
			}},
		},
		{
			name: "missing digest drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "drbd"},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "reserved sysext name drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("shadow-kubelet", deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "kubelet", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "name conflict: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("drbd-new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: otherDigest},
				}),
				nerCreated("drbd-old", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "drbd",
				Digest:      drbdDigest,
				RequestedBy: nerOwner("drbd-old"),
			}},
		},
		{
			name: "digest conflict: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("beta", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: drbdDigest},
				}),
				nerCreated("alpha", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "alpha",
				Digest:      drbdDigest,
				RequestedBy: nerOwner("alpha"),
			}},
		},
		{
			// The loser of the module contest keeps nothing: its sysext would be
			// merged onto a node running module parameters it did not ask for, and
			// its own status says which request took the module.
			name: "a request that lost a kernel module contributes nothing",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: otherDigest, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=enabled"}}}},
				}),
				nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}}},
				}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:        "alpha",
				Digest:      drbdDigest,
				RequestedBy: nerOwner("zebra"),
			}},
			wantModules: []internalv1alpha1.KernelModule{
				{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
			},
		},
		{
			name: "kernel modules collected and deduplicated by name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest, KernelModules: []deckhousev1alpha1.KernelModule{
						{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
						{Name: "drbd_transport_tcp"},
					}},
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
			extensions, modules := extensionsOf(tt.ners, tt.node, tt.ngName)
			if !reflect.DeepEqual(extensions, tt.wantExtensions) {
				t.Fatalf("extensions = %#v, want %#v", extensions, tt.wantExtensions)
			}
			if !reflect.DeepEqual(modules, tt.wantModules) {
				t.Fatalf("modules = %#v, want %#v", modules, tt.wantModules)
			}
		})
	}
}

// TestNodeExtensionsOrderDoesNotFollowTheListing: spec.extensions is an array,
// so a listing-order dependency was a spec diff — a generation bump and a
// rollout slot per node — for no change at all.
func TestNodeExtensionsOrderDoesNotFollowTheListing(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	first := nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd"}}},
	})
	second := nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: otherDigest, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "nvidia"}}},
	})

	// The same two requests, listed both ways round.
	oneWay, oneWayModules := extensionsOf([]deckhousev1alpha1.NodeExtensionRequest{first, second}, nodeWith(nil), "worker")
	otherWay, otherWayModules := extensionsOf([]deckhousev1alpha1.NodeExtensionRequest{second, first}, nodeWith(nil), "worker")

	if !reflect.DeepEqual(oneWay, otherWay) {
		t.Fatalf("extensions depend on the listing order: %#v vs %#v", oneWay, otherWay)
	}
	if !reflect.DeepEqual(oneWayModules, otherWayModules) {
		t.Fatalf("kernel modules depend on the listing order: %#v vs %#v", oneWayModules, otherWayModules)
	}

	// And it is the order every decision about them is made in: oldest first,
	// whatever the listing said.
	got := []*internalv1alpha1.ObjectOwner{oneWay[0].RequestedBy, oneWay[1].RequestedBy}
	if !reflect.DeepEqual(got, []*internalv1alpha1.ObjectOwner{nerOwner("zebra"), nerOwner("apple")}) {
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
				ner("a", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest}}),
				ner("b", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: otherDigest}}),
			},
			want: map[string]nerConflict{},
		},
		{
			name: "name conflict: older wins, newer loses on name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: otherDigest}}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"new": {reason: reasonConflict, winner: "old", field: "name"},
			},
		},
		{
			name: "digest conflict: older wins, newer loses on digest",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("new", newer, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: drbdDigest}}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"new": {reason: reasonConflict, winner: "old", field: "digest"},
			},
		},
		{
			name: "equal timestamps: lexicographically smaller name wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("zebra", same, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: otherDigest}}),
				nerCreated("apple", same, deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"zebra": {reason: reasonConflict, winner: "apple", field: "name"},
			},
		},
		{
			name: "reserved name always loses",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("shadow", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "kubelet", Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{
				"shadow": {reason: reasonReservedName},
			},
		},
		{
			name: "invalid sysext does not take part",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("no-digest", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Name: "drbd"}}),
				ner("no-name", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{Digest: drbdDigest}}),
			},
			want: map[string]nerConflict{},
		},
		{
			// One module, different parameters: deciding by listing order made the
			// running parameters depend on the alphabet and left the loser
			// claiming it had resolved.
			name: "kernel module with different parameters: the older request wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				nerCreated("apple", newer, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=enabled"}}}},
				}),
				nerCreated("zebra", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}}},
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
					Extension: deckhousev1alpha1.NodeExtension{Name: "beta", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}}},
				}),
				nerCreated("old", older, deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{Name: "alpha", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension, KernelModules: []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{"usermode_helper=disabled"}}}},
				}),
			},
			want: map[string]nerConflict{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := settleNERs(tt.ners, noPlatform)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("resolveNERConflicts = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// An Engine node merges what its requesters declared system extensions and nothing else:
// a registry package stays out of its document.
func TestTheEngineRenderTakesSystemExtensionsOnly(t *testing.T) {
	ners := []deckhousev1alpha1.NodeExtensionRequest{
		ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension},
		}),
		ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
		}),
	}
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs(ners, noPlatform)

	spec := renderSpec(immutableGroup("worker"), nodeWith(nil), in)

	require.Equal(t, []string{"drbd"}, requestedNames(spec.Extensions))
}

// noPlatform is a release whose platform document claims nothing.
var noPlatform []internalv1alpha1.Extension

// A registry package request may not take a platform package's name, which keys the
// package on the node and in the document, nor its digest, whose removal would take the
// platform's files along. A system extension of that name is no clash.
func TestARegistryPackageYieldsToThePlatform(t *testing.T) {
	for _, tc := range []struct {
		name      string
		extension deckhousev1alpha1.NodeExtension
		kind      deckhousev1alpha1.ExtensionKind
		want      map[string]nerConflict
	}{
		{
			name:      "the name of a platform package",
			extension: deckhousev1alpha1.NodeExtension{Name: "jq", Digest: drbdDigest},
			kind:      deckhousev1alpha1.ExtensionKindPackage,
			want:      map[string]nerConflict{"custom": {reason: reasonConflict, field: "name", platformPackage: "jq"}},
		},
		{
			name:      "the digest of a platform package",
			extension: deckhousev1alpha1.NodeExtension{Name: "my-jq", Digest: testenv.TestPlatformPackageDigests["jq"]},
			kind:      deckhousev1alpha1.ExtensionKindPackage,
			want:      map[string]nerConflict{"custom": {reason: reasonConflict, field: "digest", platformPackage: "jq"}},
		},
		{
			name:      "the digest of the agent's package",
			extension: deckhousev1alpha1.NodeExtension{Name: "agent-copy", Digest: testNodeletPackageDigest},
			kind:      deckhousev1alpha1.ExtensionKindPackage,
			want:      map[string]nerConflict{"custom": {reason: reasonConflict, field: "digest", platformPackage: "nodelet"}},
		},
		{
			name:      "a system extension named like a platform package",
			extension: deckhousev1alpha1.NodeExtension{Name: "jq", Digest: drbdDigest},
			kind:      deckhousev1alpha1.ExtensionKindSystemExtension,
			want:      map[string]nerConflict{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.extension.Kind = tc.kind
			request := ner("custom", deckhousev1alpha1.NodeExtensionRequestSpec{Extension: tc.extension})

			_, conflicts := settleNERs([]deckhousev1alpha1.NodeExtensionRequest{request}, testPlatform(t).Extensions)

			require.Equal(t, tc.want, conflicts)
		})
	}
}

// A request that lost to the platform claims nothing: a younger Package of its name wins
// the name and reaches the bashible node.
func TestARequestThatLostToThePlatformLeavesTheNameFree(t *testing.T) {
	older := nerCreated("a-copy", metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{
			Name: "my-jq", Digest: testenv.TestPlatformPackageDigests["jq"], Kind: deckhousev1alpha1.ExtensionKindPackage,
		}})
	younger := nerCreated("a-package", metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
		deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{
			Name: "my-jq", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage,
		}})
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{older, younger}, testPlatform(t).Extensions)

	require.Contains(t, in.NodeExtensionConflicts, "a-copy")
	require.NotContains(t, in.NodeExtensionConflicts, "a-package")
	require.Equal(t, []string{"my-jq"}, requestedNames(renderMutableSpec(namedNode("worker-0"), "worker", in).Extensions))
}

// A registry package that asks for kernel modules reaches no node, so it takes nothing
// from a younger request: the younger one was reported Conflict and went nowhere either.
func TestARequestThatReachesNoNodeClaimsNothing(t *testing.T) {
	modules := func(params string) []deckhousev1alpha1.KernelModule {
		return []deckhousev1alpha1.KernelModule{{Name: "drbd", Params: []string{params}}}
	}
	for _, tc := range []struct {
		name    string
		younger deckhousev1alpha1.NodeExtensionRequestSpec
	}{
		{
			name: "its name",
			younger: deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{
				Name: "drbd", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindPackage,
			}},
		},
		{
			name: "its digest",
			younger: deckhousev1alpha1.NodeExtensionRequestSpec{Extension: deckhousev1alpha1.NodeExtension{
				Name: "drbd-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage,
			}},
		},
		{
			name: "its kernel module",
			younger: deckhousev1alpha1.NodeExtensionRequestSpec{
				Extension: deckhousev1alpha1.NodeExtension{
					Name: "drbd-sysext", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension,
					KernelModules: modules("usermode_helper=enabled"),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			undeliverable := nerCreated("a-package", metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				deckhousev1alpha1.NodeExtensionRequestSpec{
					Extension: deckhousev1alpha1.NodeExtension{
						Name: "drbd", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage,
						KernelModules: modules("usermode_helper=disabled"),
					},
				})
			younger := nerCreated("younger", metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)), tc.younger)

			_, conflicts := settleNERs([]deckhousev1alpha1.NodeExtensionRequest{undeliverable, younger}, noPlatform)

			require.NotContains(t, conflicts, "younger")
		})
	}
}

// Names and digests are unique per kind: a Package and a SystemExtension never reach the
// same node, two Packages do. An older Package, even one selecting an Engine group, leaves
// its name to a SystemExtension there.
func TestNamesAndDigestsAreUniquePerKind(t *testing.T) {
	request := func(name string, day int, kind deckhousev1alpha1.ExtensionKind, extension, digest, group string) deckhousev1alpha1.NodeExtensionRequest {
		return nerCreated(name, metav1.NewTime(time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)), deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:         deckhousev1alpha1.NodeExtension{Name: extension, Digest: digest, Kind: kind},
			NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{group}},
		})
	}
	sysext := []internalv1alpha1.Extension{{Name: "drbd", Digest: drbdDigest, RequestedBy: nerOwner("drbd-sysext")}}
	for _, tc := range []struct {
		name string
		ners []deckhousev1alpha1.NodeExtensionRequest
		want map[string]nerConflict
		// engine is what a node of the Engine group engine gets.
		engine []internalv1alpha1.Extension
	}{
		{
			name: "a Package and a SystemExtension of one name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				request("drbd-package", 1, deckhousev1alpha1.ExtensionKindPackage, "drbd", otherDigest, "engine"),
				request("drbd-sysext", 2, deckhousev1alpha1.ExtensionKindSystemExtension, "drbd", drbdDigest, "engine"),
			},
			want:   map[string]nerConflict{},
			engine: sysext,
		},
		{
			name: "a Package and a SystemExtension of one digest",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				request("drbd-package", 1, deckhousev1alpha1.ExtensionKindPackage, "drbd-utils", drbdDigest, "worker"),
				request("drbd-sysext", 2, deckhousev1alpha1.ExtensionKindSystemExtension, "drbd", drbdDigest, "engine"),
			},
			want:   map[string]nerConflict{},
			engine: sysext,
		},
		{
			name: "two Packages of one name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				request("drbd-new", 2, deckhousev1alpha1.ExtensionKindPackage, "drbd", otherDigest, "worker"),
				request("drbd-old", 1, deckhousev1alpha1.ExtensionKindPackage, "drbd", drbdDigest, "worker"),
			},
			want: map[string]nerConflict{"drbd-new": {reason: reasonConflict, winner: "drbd-old", field: "name"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in clusterInputs
			in.NodeExtensions, in.NodeExtensionConflicts = settleNERs(tc.ners, noPlatform)

			require.Equal(t, tc.want, in.NodeExtensionConflicts)
			extensions, _ := nodeExtensions(in, deckhousev1alpha1.ExtensionKindSystemExtension, nodeWith(nil), "engine")
			require.Equal(t, tc.engine, extensions)
		})
	}
}

// Every system extension the platform puts on an Engine node is a reserved name: a request
// taking one is refused, not accepted and then dropped by the render in favour of the
// platform's (mergeExtensions). The CRD and its Russian description list the same names.
func TestEveryPlatformSystemExtensionNameIsReserved(t *testing.T) {
	platform, err := sysextDigests(map[string]map[string]string{registryPackagesDigestsKey: {
		"containerdSysext224":    "sha256:c",
		"kubernetesCniSysext162": "sha256:n",
		"kubeletSysext1356":      "sha256:k",
		"nodeletSysext":          "sha256:a",
		"qemuGuestAgentSysext":   "sha256:q",
	}}, "1.35")
	require.NoError(t, err)

	crd := testenv.NodeManagerCRDPaths(testenv.NodeExtensionRequestCRDFile)[0]
	for _, path := range []string{crd, filepath.Join(filepath.Dir(crd), "doc-ru-"+string(testenv.NodeExtensionRequestCRDFile))} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		for name := range platform {
			require.True(t, deckhousev1alpha1.IsReservedExtensionName(name), "%s is a platform extension", name)
			require.True(t, strings.Contains(string(raw), "`"+name+"`"), "%s does not list the reserved name %s", path, name)
		}
	}
}
