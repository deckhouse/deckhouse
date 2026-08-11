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

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeExtensions aggregates the NodeExtensionRequests that select this node
// into extensions and kernel modules for its NodeConfig. Invalid or conflicting
// requests are dropped, not errored — their own status carries the reason.
// The requests arrive in the uniqueness-contest order (orderedNERs), with the
// contest already settled: it is cluster-wide, so it is run once per pass rather
// than once per node, and spec.extensions must not follow the listing order.
func nodeExtensions(ordered []*deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, node *corev1.Node, ngName string) ([]internalv1alpha1.Extension, []internalv1alpha1.KernelModule) {
	// Left nil rather than empty: both marshal the same under omitempty, and
	// callers distinguish "no requests matched" by length either way.
	var extensions []internalv1alpha1.Extension
	var modules []internalv1alpha1.KernelModule
	seenModules := make(map[string]struct{})

	for _, ner := range ordered {
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}
		if _, lost := conflicts[ner.Name]; lost {
			continue
		}

		// The reason is not checked: orderedNERs has already dropped every
		// request resolveExtension could refuse.
		ext, _ := resolveExtension(ner)
		extensions = append(extensions, ext)

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

// Reasons a request is not ready, reported on its Ready condition. An empty
// reason means the extension resolved and won its sysext name and digest.
const (
	reasonResolved       = "Resolved"
	reasonInvalidSysext  = "InvalidSysext"
	reasonConflict       = "Conflict"
	reasonModuleConflict = "KernelModuleConflict"
	reasonReservedName   = "ReservedName"
	// reasonRefusedByNodes is the request that resolved here and was rejected
	// where it matters. The others are this controller's own verdicts; this one
	// is the fleet's.
	reasonRefusedByNodes = "RefusedByNodes"
)

// resolveExtension turns a request's Sysext into the NodeConfig extension the
// agent pulls through the registry-packages-proxy; fields pass straight through.
// The second return is the failure reason, empty when the extension resolved.
func resolveExtension(ner *deckhousev1alpha1.NodeExtensionRequest) (internalv1alpha1.Extension, string) {
	sysext := ner.Spec.Sysext
	if sysext.Name == "" || sysext.Digest == "" {
		return internalv1alpha1.Extension{}, reasonInvalidSysext
	}
	return internalv1alpha1.Extension{
		Name:           sysext.Name,
		Repository:     sysext.Repository,
		AdditionalPath: sysext.Path,
		Digest:         sysext.Digest,
		RequestedBy:    nerRequestedByPrefix + ner.Name,
	}, ""
}

// nerConflict records why a request lost: a reserved platform name, a
// name/digest clash with an older request (winner names it, field says which),
// or a kernel module an older request loads with different parameters.
type nerConflict struct {
	reason string
	winner string
	field  string
}

// resolveNERConflicts enforces uniqueness of sysext names, digests and kernel
// modules over requests already in orderedNERs order; the oldest request
// (creation time, then name) wins each clash. The result maps each losing
// request's name to why it lost; winners are absent.
func resolveNERConflicts(ordered []*deckhousev1alpha1.NodeExtensionRequest) map[string]nerConflict {
	nameOwner := make(map[string]string, len(ordered))
	digestOwner := make(map[string]string, len(ordered))
	moduleOwner := make(map[string]*deckhousev1alpha1.NodeExtensionRequest, len(ordered))
	conflicts := make(map[string]nerConflict)

	for _, ner := range ordered {
		sysext := ner.Spec.Sysext
		if deckhousev1alpha1.IsReservedSysextName(sysext.Name) {
			conflicts[ner.Name] = nerConflict{reason: reasonReservedName}
			continue
		}
		if owner, taken := nameOwner[sysext.Name]; taken {
			conflicts[ner.Name] = nerConflict{reason: reasonConflict, winner: owner, field: "name"}
			continue
		}
		if owner, taken := digestOwner[sysext.Digest]; taken {
			conflicts[ner.Name] = nerConflict{reason: reasonConflict, winner: owner, field: "digest"}
			continue
		}
		if conflict, lost := moduleConflict(ner, moduleOwner); lost {
			conflicts[ner.Name] = conflict
			continue
		}
		nameOwner[sysext.Name] = ner.Name
		digestOwner[sysext.Digest] = ner.Name
		for _, module := range ner.Spec.KernelModules {
			moduleOwner[module.Name] = ner
		}
	}
	return conflicts
}

// orderedNERs orders requests oldest first, ties broken by name, and drops the
// ones with an invalid sysext. One order for both the uniqueness contest and
// the node's array, so neither depends on how the requests were listed.
func orderedNERs(ners []deckhousev1alpha1.NodeExtensionRequest) []*deckhousev1alpha1.NodeExtensionRequest {
	ordered := make([]*deckhousev1alpha1.NodeExtensionRequest, 0, len(ners))
	for i := range ners {
		if ners[i].Spec.Sysext.Name == "" || ners[i].Spec.Sysext.Digest == "" {
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

	for key, value := range ner.Spec.NodeSelector.MatchLabels {
		got, ok := node.Labels[key]
		if !ok || got != value {
			return false
		}
	}
	return true
}

// mergeExtensions appends the extra extensions to the base set, dropping any
// whose name already appears in the base — the base (platform) sysexts win.
func mergeExtensions(base, extra []internalv1alpha1.Extension) []internalv1alpha1.Extension {
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
