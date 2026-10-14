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
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeExtensions collects the requests of this kind that select the node into its NodeConfig.
// Invalid and losing requests are dropped, not errored: their status says why. The contest
// is cluster-wide and settled once per pass (settleNERs), not once per node.
func nodeExtensions(in clusterInputs, kind deckhousev1alpha1.ExtensionKind, node *corev1.Node, ngName string) ([]internalv1alpha1.Extension, []internalv1alpha1.KernelModule) {
	// Left nil rather than empty: both marshal the same under omitempty, and
	// callers distinguish "no requests matched" by length either way.
	var extensions []internalv1alpha1.Extension
	var modules []internalv1alpha1.KernelModule
	seenModules := make(map[string]struct{})

	for _, ner := range in.NodeExtensions {
		if !deliverable(ner, kind) {
			continue
		}
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}
		if _, lost := in.NodeExtensionConflicts[ner.Name]; lost {
			continue
		}

		// Validity is not checked here: settleNERs has already dropped every
		// request with an unusable extension.
		extensions = append(extensions, resolveExtension(ner))

		for _, module := range ner.Spec.KernelModules {
			if _, seen := seenModules[module.Name]; seen {
				continue
			}
			seenModules[module.Name] = struct{}{}
			modules = append(modules, internalv1alpha1.KernelModule{
				Name:   module.Name,
				Params: module.Params,
			})
		}
	}

	return extensions, modules
}

// deliverable reports whether a request goes to the nodes that install artifacts of this
// kind: it must ask for one, and a registry package cannot load kernel modules.
func deliverable(ner *deckhousev1alpha1.NodeExtensionRequest, nodes deckhousev1alpha1.ExtensionKind) bool {
	if ner.Spec.Extension.Kind != nodes {
		return false
	}
	return nodes == deckhousev1alpha1.ExtensionKindSystemExtension || len(ner.Spec.KernelModules) == 0
}

// Reasons a request is not ready, reported on its Ready condition. An empty
// reason means the extension resolved and won its name and digest.
const (
	reasonResolved         = "Resolved"
	reasonInvalidExtension = "InvalidExtension"
	reasonConflict         = "Conflict"
	reasonModuleConflict   = "KernelModuleConflict"
	reasonReservedName     = "ReservedName"
	// reasonRefusedByNodes is the request that resolved here and was rejected
	// where it matters. The others are this controller's own verdicts; this one
	// is the fleet's.
	reasonRefusedByNodes = "RefusedByNodes"
	// reasonNotDeliverable is a request whose selected groups cannot install its kind
	// of image, or a registry package that asks for kernel modules.
	reasonNotDeliverable = "NotDeliverable"
)

// extensionValid reports whether a request names an image the nodes can pull: both
// name and digest are needed. The one refusal this controller resolves on.
func extensionValid(ner *deckhousev1alpha1.NodeExtensionRequest) bool {
	return ner.Spec.Extension.Name != "" && ner.Spec.Extension.Digest != ""
}

// resolveExtension turns a request's extension into the NodeConfig extension the
// agent pulls through the registry-packages-proxy; fields pass straight through.
// Callers filter on extensionValid first.
func resolveExtension(ner *deckhousev1alpha1.NodeExtensionRequest) internalv1alpha1.Extension {
	ext := ner.Spec.Extension
	return internalv1alpha1.Extension{
		Name:           ext.Name,
		Repository:     ext.Repository,
		AdditionalPath: ext.Path,
		Digest:         ext.Digest,
		RequestedBy:    nerRequestedByPrefix + ner.Name,
	}
}

// nerConflict records why a request lost: a reserved platform name; a platform package's
// name or digest (platformPackage names the package); a name/digest clash with an older
// request (winner names it, field says which); a kernel module loaded differently.
type nerConflict struct {
	reason          string
	winner          string
	field           string
	platformPackage string
}

// resolveNERConflicts maps each request losing a name, digest or kernel module to why; the
// oldest in orderedNERs order wins. Names and digests are unique per kind: a Package reaches
// bashible nodes only, a SystemExtension Engine ones, and systemType never changes once set.
func resolveNERConflicts(ordered []*deckhousev1alpha1.NodeExtensionRequest, platform []internalv1alpha1.Extension) map[string]nerConflict {
	nameOwner := make(map[string]string, len(ordered))
	digestOwner := make(map[string]string, len(ordered))
	moduleOwner := make(map[string]*deckhousev1alpha1.NodeExtensionRequest, len(ordered))
	conflicts := make(map[string]nerConflict)

	for _, ner := range ordered {
		ext := ner.Spec.Extension
		if deckhousev1alpha1.IsReservedExtensionName(ext.Name) {
			conflicts[ner.Name] = nerConflict{reason: reasonReservedName}
			continue
		}
		if conflict, lost := platformConflict(ext, platform); lost {
			conflicts[ner.Name] = conflict
			continue
		}
		nameKey := string(ext.Kind) + "/" + ext.Name
		digestKey := string(ext.Kind) + "/" + ext.Digest
		if owner, taken := nameOwner[nameKey]; taken {
			conflicts[ner.Name] = nerConflict{reason: reasonConflict, winner: owner, field: "name"}
			continue
		}
		if owner, taken := digestOwner[digestKey]; taken {
			conflicts[ner.Name] = nerConflict{reason: reasonConflict, winner: owner, field: "digest"}
			continue
		}
		if conflict, lost := moduleConflict(ner, moduleOwner); lost {
			conflicts[ner.Name] = conflict
			continue
		}
		// deliverable() takes the request's own kind only, so this is "reaches no node":
		// such a request must not take a name, digest or module from a younger one.
		if !deliverable(ner, ext.Kind) {
			continue
		}
		nameOwner[nameKey] = ner.Name
		digestOwner[digestKey] = ner.Name
		for _, module := range ner.Spec.KernelModules {
			moduleOwner[module.Name] = ner
		}
	}
	return conflicts
}

// platformConflict reports a registry package request that takes the name or digest of a
// package the platform installs on the same nodes: the name keys the package there, and
// removing the request would remove the platform's files. A system extension takes neither.
func platformConflict(ext deckhousev1alpha1.NodeExtension, platform []internalv1alpha1.Extension) (nerConflict, bool) {
	if ext.Kind != deckhousev1alpha1.ExtensionKindPackage {
		return nerConflict{}, false
	}
	for _, claim := range platform {
		if ext.Name == claim.Name {
			return nerConflict{reason: reasonConflict, field: "name", platformPackage: claim.Name}, true
		}
		if ext.Digest == claim.Digest {
			return nerConflict{reason: reasonConflict, field: "digest", platformPackage: claim.Name}, true
		}
	}
	return nerConflict{}, false
}

// orderedNERs orders requests oldest first, ties broken by name, and drops the
// ones with an invalid extension. One order for both the uniqueness contest and
// the node's array, so neither depends on how the requests were listed.
func orderedNERs(ners []deckhousev1alpha1.NodeExtensionRequest) []*deckhousev1alpha1.NodeExtensionRequest {
	ordered := make([]*deckhousev1alpha1.NodeExtensionRequest, 0, len(ners))
	for i := range ners {
		if !extensionValid(&ners[i]) {
			continue
		}
		ordered = append(ordered, &ners[i])
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

// settleNERs is what every render and the status pass take from the requests: the valid
// ones in contest order, and which ones lost the contest to an older request or to the
// platform packages of the release.
func settleNERs(ners []deckhousev1alpha1.NodeExtensionRequest, platform []internalv1alpha1.Extension) ([]*deckhousev1alpha1.NodeExtensionRequest, map[string]nerConflict) {
	ordered := orderedNERs(ners)
	return ordered, resolveNERConflicts(ordered, platform)
}

// moduleConflict reports whether a request asks for a kernel module an earlier
// one already loads with different parameters; the same module with the same
// parameters is not a clash and is loaded once.
func moduleConflict(ner *deckhousev1alpha1.NodeExtensionRequest, owners map[string]*deckhousev1alpha1.NodeExtensionRequest) (nerConflict, bool) {
	for _, module := range ner.Spec.KernelModules {
		owner, taken := owners[module.Name]
		if !taken || sameModuleParams(owner, module) {
			continue
		}
		return nerConflict{reason: reasonModuleConflict, winner: owner.Name, field: module.Name}, true
	}
	return nerConflict{}, false
}

// sameModuleParams reports whether the owner of a module loads it with exactly
// the parameters this request asks for.
func sameModuleParams(owner *deckhousev1alpha1.NodeExtensionRequest, module deckhousev1alpha1.KernelModule) bool {
	for _, owned := range owner.Spec.KernelModules {
		if owned.Name != module.Name {
			continue
		}
		return slices.Equal(owned.Params, module.Params)
	}
	return false
}

// nerMatchesNode reports whether the request selects this node: its NodeGroup
// must be listed (or the list left empty) and every requested node label must be
// present on the node with the same value.
func nerMatchesNode(ner *deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string) bool {
	names := ner.Spec.NodeGroupSelector.MatchNames
	if len(names) > 0 && !slices.Contains(names, ngName) {
		return false
	}

	selector := labels.SelectorFromValidatedSet(ner.Spec.NodeSelector.MatchLabels)
	return selector.Matches(labels.Set(node.Labels))
}

// mergeExtensions appends the extra extensions to the base set, dropping any
// whose name already appears in the base — the base (platform) sysexts win.
func mergeExtensions(base, extra []internalv1alpha1.Extension) []internalv1alpha1.Extension {
	// The base is the pass's memoised platform document, shared by every node it renders.
	base = slices.Clone(base)
	seen := make(map[string]struct{}, len(base))
	for _, ext := range base {
		seen[ext.Name] = struct{}{}
	}
	for _, ext := range extra {
		if _, ok := seen[ext.Name]; ok {
			continue
		}
		seen[ext.Name] = struct{}{}
		base = append(base, ext)
	}
	return base
}
