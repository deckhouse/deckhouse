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

	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// maxStaticPods is what NodeSpec.StaticPods accepts (MaxItems=16 in
// crds/nodeconfig.yaml). Beyond it the API server refuses the whole config and
// the node is left with none, so the render drops the surplus instead.
const maxStaticPods = 16

// staticPodLog reports the objects a node config had no room for. renderSpec is
// pure and takes no logger, and the drop is a misconfiguration an operator has
// to see.
var staticPodLog = log.Log.WithName(controllerName)

// reasonInvalidManifest is the one refusal a static pod has that an extension
// request does not; the rest of the vocabulary (Resolved, ReservedName, Conflict,
// RefusedByNodes) is shared and declared in extensions.go.
const reasonInvalidManifest = "InvalidManifest"

// reasonInvalidName is the other: an object name the API server admits — a CR
// name is a DNS subdomain — and spec.staticPods[].name does not.
const reasonInvalidName = "InvalidName"

// reasonLimitExceeded is an accepted object the per-group cap (maxStaticPods)
// left out of a node config: nothing is wrong with it, it simply never arrives.
const reasonLimitExceeded = "LimitExceeded"

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

// rejectedNSPRs runs the checks in order (a name the node config would not take,
// reserved name, invalid manifest, reserved pod, pod already claimed by an older object);
// the contest is cluster-wide, as in resolveNERConflicts, and a refusal claims nothing.
func rejectedNSPRs(ordered []*deckhousev1alpha1.NodeStaticPodRequest) map[string]nsprRefusal {
	rejected := map[string]nsprRefusal{}
	claimed := make(map[string]string, len(ordered))

	for _, nspr := range ordered {
		if err := deckhousev1alpha1.ValidateStaticPodName(nspr.Name); err != nil {
			rejected[nspr.Name] = nsprRefusal{reason: reasonInvalidName, message: err.Error()}
			continue
		}
		if deckhousev1alpha1.IsReservedStaticPodName(nspr.Name) {
			rejected[nspr.Name] = nsprRefusal{
				reason:  reasonReservedName,
				message: "the name belongs to a manifest the node agent or a bashible step writes itself",
			}
			continue
		}
		pod, err := deckhousev1alpha1.ValidateStaticPodManifest(nspr.Spec.Manifest)
		if err != nil {
			rejected[nspr.Name] = nsprRefusal{reason: reasonInvalidManifest, message: err.Error()}
			continue
		}
		if deckhousev1alpha1.IsReservedStaticPod(pod) {
			rejected[nspr.Name] = nsprRefusal{
				reason:  reasonReservedName,
				message: fmt.Sprintf("the pod %s already has a manifest the node agent or a bashible step writes itself", pod),
			}
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
// Beyond maxStaticPods the surplus is dropped in contest order, youngest first:
// the alternative is a spec the API server refuses whole.
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

	if len(pods) > maxStaticPods {
		dropped := make([]string, 0, len(pods)-maxStaticPods)
		for _, pod := range pods[maxStaticPods:] {
			dropped = append(dropped, pod.Name)
		}
		staticPodLog.Error(nil, "more static pods select the group than a node config holds; the youngest are left out",
			"nodeGroup", ngName, "limit", maxStaticPods, "dropped", dropped)
		pods = pods[:maxStaticPods]
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

// cappedNSPRs maps every object the cap left out of a group's node configs to
// those groups, by asking the render what it kept (nodeStaticPods).
func cappedNSPRs(ordered []*deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, groups []string) map[string][]string {
	capped := map[string][]string{}
	for _, group := range groups {
		kept := nodeStaticPods(ordered, rejected, group)
		for _, nspr := range ordered {
			if !nsprMatchesNodeGroup(nspr, group) {
				continue
			}
			if _, refused := rejected[nspr.Name]; refused {
				continue
			}
			if slices.ContainsFunc(kept, func(pod internalv1alpha1.StaticPod) bool { return pod.Name == nspr.Name }) {
				continue
			}
			capped[nspr.Name] = append(capped[nspr.Name], group)
		}
	}
	return capped
}
