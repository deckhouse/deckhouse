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

// defaultModuleSourceName is the ModuleSource a request targets when it does not
// name one via the "MODULE_SOURCE" param — the canonical Deckhouse source.
const defaultModuleSourceName = "deckhouse"

// placeholderPattern matches an unresolved ${KEY} left in an image reference.
var placeholderPattern = regexp.MustCompile(`\$\{[^}]+\}`)

// nodeExtensions aggregates the NodeExtensionRequests that select this node into
// the extensions and kernel modules to add to its NodeConfig. It is a pure
// function of its inputs so the matching and template resolution can be tested
// without a cluster. A request is dropped (not surfaced as an error) when its
// image template cannot be fully resolved or no digest is known for it.
func nodeExtensions(ners []deckhousev1alpha1.NodeExtensionRequest, node *corev1.Node, ngName string, digests map[string]string, kernelVersion string, moduleSourceRepos map[string]string) (extensions []internalv1alpha1.Extension, modules []internalv1alpha1.KernelModule) {
	seenExtensions := make(map[string]struct{})
	seenModules := make(map[string]struct{})

	for i := range ners {
		ner := &ners[i]
		if !nerMatchesNode(ner, node, ngName) {
			continue
		}

		msRepo := moduleSourceRepo(ner, moduleSourceRepos)
		ref := resolveImageTemplate(ner.Spec.ImageTemplate, ner.Spec.Params, kernelVersion, msRepo)
		if placeholderPattern.MatchString(ref) {
			// A placeholder with no value cannot be resolved to an image.
			continue
		}

		ref, pinnedDigest := splitDigest(ref)
		repository, additionalPath, name := splitReference(ref, msRepo)
		if name == "" {
			continue
		}
		// A request may pin the digest in its image template (…@sha256:…); that
		// digest wins. Otherwise the release digest map (base extensions) is
		// consulted. Without either, the image cannot be pulled, so drop it.
		digest := pinnedDigest
		if digest == "" {
			digest = digests[name]
		}
		if digest == "" {
			continue
		}

		if _, ok := seenExtensions[name]; !ok {
			seenExtensions[name] = struct{}{}
			extensions = append(extensions, internalv1alpha1.Extension{
				Name:           name,
				Repository:     repository,
				AdditionalPath: additionalPath,
				Digest:         digest,
				RequestedBy:    requestedBy(ner),
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
// template: the reserved KERNEL_VERSION and MODULE_SOURCE_REPO first, then the
// request's own params. MODULE_SOURCE_REPO is left in place when its repo is
// unknown so the caller drops the request rather than emitting a malformed ref.
func resolveImageTemplate(template string, params map[string]string, kernelVersion, moduleSourceRepo string) string {
	ref := strings.ReplaceAll(template, "${KERNEL_VERSION}", kernelVersion)
	if moduleSourceRepo != "" {
		ref = strings.ReplaceAll(ref, "${MODULE_SOURCE_REPO}", moduleSourceRepo)
	}
	for key, value := range params {
		ref = strings.ReplaceAll(ref, "${"+key+"}", value)
	}
	return ref
}

// moduleSourceRepo returns the registry repo of the ModuleSource a request
// targets. Its "MODULE_SOURCE" param names the source, defaulting to the
// canonical "deckhouse" source. Empty when the source is unknown, which leaves
// ${MODULE_SOURCE_REPO} unresolved and drops the request.
func moduleSourceRepo(ner *deckhousev1alpha1.NodeExtensionRequest, repos map[string]string) string {
	name := ner.Spec.Params["MODULE_SOURCE"]
	if name == "" {
		name = defaultModuleSourceName
	}
	return repos[name]
}

// splitDigest separates a pinned "@sha256:<hex>" digest from the reference. NER
// targets module images, which live at a per-module registry path rather than in
// the release digest map; pinning the digest in the template is how such an image
// is addressed. When present the pinned digest is authoritative.
func splitDigest(ref string) (base, digest string) {
	if idx := strings.LastIndex(ref, "@"); idx >= 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}

// splitReference separates a resolved image reference (already stripped of any
// pinned "@digest") into the three fields the packages proxy consumes:
//   - repository: the key the proxy resolves registry auth by — the ModuleSource
//     repo the image lives under (registry-packages-proxy keys its per-source
//     credentials by ModuleSource.spec.registry.repo). The proxy then fetches
//     "<repository>/<additionalPath>@<digest>".
//   - additionalPath: the sub-repo under that ModuleSource repo, minus the name.
//   - name: the LAST segment, a logical sysext name matched against the image's
//     extension-release, NOT part of the registry path (the digest, not the
//     name, locates the manifest).
//
// With moduleSourceRepo "dev-registry.io/sys/deckhouse-oss/modules", the ref
// "dev-registry.io/sys/deckhouse-oss/modules/sds-replicated-volume/drbd" yields
// repository = the ModuleSource repo, additionalPath "sds-replicated-volume",
// name "drbd". A reference not anchored to the ModuleSource repo falls back to a
// host/path split (repository = host), which the proxy resolves only when a
// matching per-registry config exists. A trailing ":tag" is stripped when it is
// a tag rather than a host's ":port", told apart by a following "/".
func splitReference(ref, moduleSourceRepo string) (repository, additionalPath, name string) {
	if idx := strings.LastIndex(ref, ":"); idx >= 0 && !strings.Contains(ref[idx+1:], "/") {
		ref = ref[:idx]
	}
	if moduleSourceRepo != "" && strings.HasPrefix(ref, moduleSourceRepo+"/") {
		additionalPath, name = splitLast(strings.TrimPrefix(ref, moduleSourceRepo+"/"))
		return moduleSourceRepo, additionalPath, name
	}
	repository = ref
	rest := ""
	if idx := strings.Index(ref, "/"); idx >= 0 {
		repository, rest = ref[:idx], ref[idx+1:]
	}
	additionalPath, name = splitLast(rest)
	if name == "" {
		name = repository
	}
	return repository, additionalPath, name
}

// splitLast splits a slash path into everything before the last segment and the
// last segment itself.
func splitLast(path string) (prefix, last string) {
	last = path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		prefix, last = path[:idx], path[idx+1:]
	}
	return prefix, last
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
