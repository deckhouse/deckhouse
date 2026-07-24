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
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// moduleNameLabel records which Deckhouse module owns a NodeExtensionRequest.
// It becomes the extension's RequestedBy so the on-node agent can attribute the
// sysext to the module that asked for it.
const moduleNameLabel = "module.deckhouse.io/name"

// placeholderPattern matches an unresolved ${KEY} left in an image reference.
var placeholderPattern = regexp.MustCompile(`\$\{[^}]+\}`)

// nodeExtensions aggregates the NodeExtensionRequests that select this node into
// the extensions and kernel modules to add to its NodeConfig. It is a pure
// function of its inputs so the matching and template resolution can be tested
// without a cluster. A request is dropped (not surfaced as an error) when its
// image template cannot be fully resolved or no digest is known for it.
func nodeExtensions(ners []deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string, digests map[string]string, kernelVersion string) (extensions []internalv1alpha1.Extension, modules []internalv1alpha1.KernelModule) {
	seenExtensions := make(map[string]struct{})
	seenModules := make(map[string]struct{})

	for i := range ners {
		ner := &ners[i]
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}

		ref := resolveImageTemplate(ner.Spec.ImageTemplate, ner.Spec.Params, kernelVersion)
		if placeholderPattern.MatchString(ref) {
			// A placeholder with no value cannot be resolved to an image.
			continue
		}

		repository, name := splitReference(ref)
		if name == "" {
			continue
		}
		digest := digests[name]
		if digest == "" {
			continue
		}

		if _, ok := seenExtensions[name]; !ok {
			seenExtensions[name] = struct{}{}
			extensions = append(extensions, internalv1alpha1.Extension{
				Name:        name,
				Repository:  repository,
				Digest:      digest,
				RequestedBy: requestedBy(ner),
			})
		}

		for _, module := range ner.Spec.KernelModules {
			if _, ok := seenModules[module.Name]; ok {
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

// resolveImageTemplate substitutes the ${KEY} placeholders in the image
// template: the reserved KERNEL_VERSION first, then the request's own params.
func resolveImageTemplate(template string, params map[string]string, kernelVersion string) string {
	ref := strings.ReplaceAll(template, "${KERNEL_VERSION}", kernelVersion)
	for key, value := range params {
		ref = strings.ReplaceAll(ref, "${"+key+"}", value)
	}
	return ref
}

// splitReference separates a resolved image reference into its repository and
// the sysext name (the repository's basename). The trailing ":tag" is stripped
// only when it is a tag rather than the ":port" of a registry host, told apart
// by whether a path ("/") follows the colon.
func splitReference(ref string) (repository, name string) {
	repository = ref
	if idx := strings.LastIndex(ref, ":"); idx >= 0 && !strings.Contains(ref[idx+1:], "/") {
		repository = ref[:idx]
	}
	name = repository
	if idx := strings.LastIndex(repository, "/"); idx >= 0 {
		name = repository[idx+1:]
	}
	return repository, name
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
