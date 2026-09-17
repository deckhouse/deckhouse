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
	"fmt"
	"slices"
	"sort"
	"strings"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// reasonInvalidManifest is the one refusal a static pod has that an extension
// request does not; the rest of the vocabulary (Resolved, ReservedName, Conflict,
// RefusedByNodes) is shared and declared in extensions.go.
const reasonInvalidManifest = "InvalidManifest"

// nsprRefusal records why a static pod was refused: the reason its Ready
// condition carries, and the message that says what is wrong with it.
type nsprRefusal struct {
	reason  string
	message string
}

// orderedNSPRs orders the requests oldest first, ties broken by name. One order
// for the pod contest and for what each node is given, so neither depends on how
// the objects were listed. Mirrors orderedNERs (extensions.go), minus its
// validity filter: an object with a bad manifest still needs a status, and it
// gets one from rejectedNSPRs rather than by disappearing here.
func orderedNSPRs(nsprs []deckhousev1alpha1.NodeStaticPodRequest) []*deckhousev1alpha1.NodeStaticPodRequest {
	ordered := make([]*deckhousev1alpha1.NodeStaticPodRequest, 0, len(nsprs))
	for i := range nsprs {
		ordered = append(ordered, &nsprs[i])
	}
	sort.Slice(ordered, func(i, j int) bool {
		ti, tj := ordered[i].CreationTimestamp, ordered[j].CreationTimestamp
		if !ti.Equal(&tj) {
			return ti.Before(&tj)
		}
		return ordered[i].Name < ordered[j].Name
	})
	return ordered
}

// rejectedNSPRs settles, once per pass, everything this controller can decide
// before a node sees the object. Three checks in this order, because each only
// makes sense once the one before it passed:
//
//  1. the object's own name belongs to a control-plane manifest the node agent
//     writes itself;
//  2. the manifest is not a valid Pod, or names no pod at all;
//  3. the pod it names was already asked for by an older object.
//
// The third is the only contest between objects, and it is cluster-wide the way
// the extension requests' is (resolveNERConflicts, extensions.go). Two entries
// for one namespace/metadata.name in one NodeConfig is a document the node's
// loader refuses whole — the node would lose every other static pod with it.
// Settling it per node would be laxer, since two objects selecting disjoint
// groups never meet, but it would also let one object be applied on one node and
// refused on another, with a single status field to say so.
//
// A refused object claims nothing, so the pod it named stays free for the next
// one: an object that lost on its own name must not take a pod down with it.
func rejectedNSPRs(ordered []*deckhousev1alpha1.NodeStaticPodRequest) map[string]nsprRefusal {
	rejected := map[string]nsprRefusal{}
	claimed := make(map[string]string, len(ordered))

	for _, nspr := range ordered {
		if deckhousev1alpha1.IsReservedStaticPodName(nspr.Name) {
			rejected[nspr.Name] = nsprRefusal{
				reason:  reasonReservedName,
				message: "the name belongs to the control plane, whose manifests the node agent writes itself",
			}
			continue
		}
		pod, err := deckhousev1alpha1.ValidateStaticPodManifest(nspr.Spec.Manifest)
		if err != nil {
			rejected[nspr.Name] = nsprRefusal{reason: reasonInvalidManifest, message: err.Error()}
			continue
		}
		if owner, taken := claimed[pod]; taken {
			rejected[nspr.Name] = nsprRefusal{
				reason: reasonConflict,
				message: fmt.Sprintf(
					"the pod %s is already asked for by NodeStaticPodRequest %q, which is older; two manifests for one pod in a node's config are refused by the node as a whole",
					pod, owner),
			}
			continue
		}
		claimed[pod] = nspr.Name
	}
	return rejected
}

// nodeStaticPods returns the static pods a node runs: the accepted objects that
// select the node's group, sorted by name so the rendered spec does not follow
// the listing order. The entry's name is the object's — the manifest's file name
// on the node — and the pod inside it is called whatever the manifest says.
func nodeStaticPods(ordered []*deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, ngName string) []internalv1alpha1.StaticPod {
	// Left nil rather than empty: both marshal the same under omitempty.
	var pods []internalv1alpha1.StaticPod

	for _, nspr := range ordered {
		if !nsprMatchesNodeGroup(nspr, ngName) {
			continue
		}
		if _, refused := rejected[nspr.Name]; refused {
			continue
		}
		pods = append(pods, internalv1alpha1.StaticPod{
			Name:     nspr.Name,
			Manifest: nspr.Spec.Manifest,
		})
	}

	slices.SortFunc(pods, func(a, b internalv1alpha1.StaticPod) int {
		return strings.Compare(a.Name, b.Name)
	})
	return pods
}

// nsprMatchesNodeGroup reports whether the static pod selects this node's group;
// naming no group selects every one of them.
func nsprMatchesNodeGroup(nspr *deckhousev1alpha1.NodeStaticPodRequest, ngName string) bool {
	names := nspr.Spec.NodeGroupSelector.MatchNames
	return len(names) == 0 || slices.Contains(names, ngName)
}
