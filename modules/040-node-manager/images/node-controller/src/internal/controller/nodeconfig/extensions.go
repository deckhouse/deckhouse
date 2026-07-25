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
	corev1 "k8s.io/api/core/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// moduleNameLabel records which Deckhouse module owns a NodeExtensionRequest.
// It becomes the extension's RequestedBy so the on-node agent can attribute the
// sysext to the module that asked for it.
const moduleNameLabel = "module.deckhouse.io/name"

// defaultModuleSourceName is the ModuleSource a request targets when its Sysext
// does not name one — the canonical Deckhouse source.
const defaultModuleSourceName = "deckhouse"

// nodeExtensions aggregates the NodeExtensionRequests that select this node into
// the extensions and kernel modules to add to its NodeConfig. It is a pure
// function of its inputs so the matching can be tested without a cluster. A
// request is dropped (not surfaced as an error) when its sysext cannot be
// resolved (unknown ModuleSource, or no path to locate the image).
func nodeExtensions(ners []deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string, moduleSourceRepos map[string]string) (extensions []internalv1alpha1.Extension, modules []internalv1alpha1.KernelModule) {
	seenExtensions := make(map[string]struct{})
	seenModules := make(map[string]struct{})

	for i := range ners {
		ner := &ners[i]
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}

		ext, reason := resolveExtension(ner, moduleSourceRepos)
		if reason != "" {
			continue
		}
		if _, seen := seenExtensions[ext.Name]; !seen {
			seenExtensions[ext.Name] = struct{}{}
			extensions = append(extensions, ext)
		}

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

// Reasons a request's sysext cannot be resolved, reported on its Ready
// condition. An empty reason means the extension resolved.
const (
	reasonResolved      = "Resolved"
	reasonInvalidSysext = "InvalidSysext"
	reasonUnknownSource = "UnknownModuleSource"
	reasonNoPath        = "NoPath"
)

// resolveExtension turns a request's Sysext into the NodeConfig extension the
// on-node agent pulls through the registry-packages-proxy. Repository is the
// ModuleSource repo — the key the proxy resolves credentials by, so the image is
// fetched with that source's auth — and AdditionalPath is the repo path under
// it, defaulting to the module name. The second return is the failure reason, or
// empty when the extension resolved.
func resolveExtension(ner *deckhousev1alpha1.NodeExtensionRequest, moduleSourceRepos map[string]string) (internalv1alpha1.Extension, string) {
	sysext := ner.Spec.Sysext
	if sysext.Name == "" || sysext.Digest == "" {
		return internalv1alpha1.Extension{}, reasonInvalidSysext
	}

	sourceName := sysext.ModuleSource
	if sourceName == "" {
		sourceName = defaultModuleSourceName
	}
	repository := moduleSourceRepos[sourceName]
	if repository == "" {
		return internalv1alpha1.Extension{}, reasonUnknownSource
	}

	path := sysext.Path
	if path == "" {
		path = ner.Labels[moduleNameLabel]
	}
	if path == "" {
		return internalv1alpha1.Extension{}, reasonNoPath
	}

	return internalv1alpha1.Extension{
		Name:           sysext.Name,
		Repository:     repository,
		AdditionalPath: path,
		Digest:         sysext.Digest,
		RequestedBy:    requestedBy(ner),
	}, ""
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

// requestedBy is the owner recorded on the extension: the module that created
// the request, falling back to the request's own name.
func requestedBy(ner *deckhousev1alpha1.NodeExtensionRequest) string {
	if module := ner.Labels[moduleNameLabel]; module != "" {
		return module
	}
	return ner.Name
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
