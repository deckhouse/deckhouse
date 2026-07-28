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
	"sort"

	corev1 "k8s.io/api/core/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeExtensions aggregates the NodeExtensionRequests that select this node into
// the extensions and kernel modules to add to its NodeConfig. It is a pure
// function of its inputs so the matching can be tested without a cluster. A
// request is dropped (not surfaced as an error) when its sysext is invalid or it
// lost the name/digest uniqueness contest to another request (see
// resolveNERConflicts) — its own status carries the reason.
func nodeExtensions(ners []deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string) ([]internalv1alpha1.Extension, []internalv1alpha1.KernelModule) {
	// Left nil rather than empty: both marshal the same under omitempty, and
	// callers distinguish "no requests matched" by length either way.
	var extensions []internalv1alpha1.Extension
	var modules []internalv1alpha1.KernelModule
	conflicts := resolveNERConflicts(ners)
	seenModules := make(map[string]struct{})

	for i := range ners {
		ner := &ners[i]
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}
		if _, lost := conflicts[ner.Name]; lost {
			continue
		}

		ext, reason := resolveExtension(ner)
		if reason != "" {
			continue
		}
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
	reasonResolved      = "Resolved"
	reasonInvalidSysext = "InvalidSysext"
	reasonConflict      = "Conflict"
	reasonReservedName  = "ReservedName"
)

// resolveExtension turns a request's Sysext into the NodeConfig extension the
// on-node agent pulls through the registry-packages-proxy. The sysext fields pass
// straight through: Repository is the proxy's credential key and AdditionalPath
// the path within it, both optional. The second return is the failure reason, or
// empty when the extension resolved.
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
		RequestedBy:    ner.Name,
	}, ""
}

// nerConflict records why a request lost its sysext: it either reused a platform
// name (reasonReservedName) or clashed with an older request on its sysext name
// or digest (reasonConflict — winner names that request, field the clashing one).
type nerConflict struct {
	reason string
	winner string
	field  string
}

// resolveNERConflicts enforces sysext uniqueness across all requests: each sysext
// name and each digest backs at most one request. The winner of a clash is the
// oldest request (by creation time, then name); every later request claiming the
// same name or digest loses, as does any request whose name is reserved for a
// platform extension. The result maps each losing request's name to why it lost;
// winners are absent. Requests with an invalid sysext (no name or digest) do not
// take part — resolveExtension reports those.
func resolveNERConflicts(ners []deckhousev1alpha1.NodeExtensionRequest) map[string]nerConflict {
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

	nameOwner := make(map[string]string, len(ordered))
	digestOwner := make(map[string]string, len(ordered))
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
		nameOwner[sysext.Name] = ner.Name
		digestOwner[sysext.Digest] = ner.Name
	}
	return conflicts
}

// nerMatchesNode reports whether the request selects this node: its NodeGroup
// must be listed (or the list left empty) and every requested node label must be
// present on the node with the same value.
func nerMatchesNode(ner *deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string) bool {
	names := ner.Spec.NodeGroupSelector.MatchNames
	if len(names) > 0 {
		matched := false
		for _, name := range names {
			if name == ngName {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
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

// mergeModules appends the extra kernel modules to the base set, deduplicated by
// name with the base winning.
func mergeModules(base, extra []internalv1alpha1.KernelModule) []internalv1alpha1.KernelModule {
	seen := make(map[string]struct{}, len(base))
	for _, module := range base {
		seen[module.Name] = struct{}{}
	}
	for _, module := range extra {
		if _, ok := seen[module.Name]; ok {
			continue
		}
		seen[module.Name] = struct{}{}
		base = append(base, module)
	}
	return base
}
