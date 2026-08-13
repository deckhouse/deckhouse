// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package immutable

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// ValidateSysext reports whether the installer image carries the system
// extensions an immutable node runs; a preflight check calls it to fail early.
// Pure; the context is here for the package's uniform exported signature.
func ValidateSysext(_ context.Context, metaConfig *config.MetaConfig) error {
	version, err := kubernetesVersion(metaConfig)
	if err != nil {
		return err
	}
	_, err = sysextExtensions(metaConfig.Images.ConvertToMap(), version)
	return err
}

// sysextExtensions looks the extensions up in images_digests.json. The image
// names there are produced by the sprig camelcase function, which strips the
// separators: kubelet-sysext-1-34-9 becomes registrypackages.kubeletSysext1349.
func sysextExtensions(images map[string]any, kubernetesVersion string) ([]extension, error) {
	packages, err := digestGroup(images, registryPackagesDigestsKey)
	if err != nil {
		return nil, err
	}

	containerd, err := soleDigest(packages, "containerdSysext")
	if err != nil {
		return nil, err
	}
	cni, err := soleDigest(packages, "kubernetesCniSysext")
	if err != nil {
		return nil, err
	}
	minor := strings.ReplaceAll(kubernetesVersion, ".", "")

	// The order is the payload's: the node writes them in it, and every rendered
	// document since the first one has carried them this way.
	extensions := []extension{
		{Name: containerdExtension, Digest: containerd, RequestedBy: platformExtensionRequestedBy},
		{Name: kubeletExtension, Digest: newestPatchDigest(packages, "kubeletSysext"+minor), RequestedBy: platformExtensionRequestedBy},
		{Name: cniExtension, Digest: cni, RequestedBy: platformExtensionRequestedBy},
	}
	for _, e := range extensions {
		if e.Digest == "" {
			return nil, fmt.Errorf(
				"the installer image carries no %q system extension digest for Kubernetes %s",
				e.Name, kubernetesVersion,
			)
		}
	}

	return extensions, nil
}

// versionedImages returns the images named the prefix followed by a version, as
// image name to version. Everything after the prefix is the version, so a
// non-numeric tail is another image whose name merely starts the same way.
func versionedImages(packages map[string]string, prefix string) map[string]int {
	found := map[string]int{}
	for name := range packages {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		version, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		found[name] = version
	}
	return found
}

// soleDigest returns the digest of the one image with the given prefix. No
// "newest" can be told: camelcase strips separators, so "kubernetesCniSysext1610"
// is ambiguous. Kept in step with node-controller's nodeconfig/sources.go.
func soleDigest(packages map[string]string, prefix string) (string, error) {
	found := slices.Sorted(maps.Keys(versionedImages(packages, prefix)))

	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return packages[found[0]], nil
	default:
		return "", fmt.Errorf(
			"the installer image carries %d %q system extensions (%s): their names do not say which one is newer. "+
				"That is a defect in the installer image, which is built to ship exactly one of each; "+
				"nothing in the cluster configuration causes it and nothing in it can work around it",
			len(found), prefix, strings.Join(found, ", "),
		)
	}
}

// newestPatchDigest returns the newest image with the given prefix, which pins
// everything but the patch — so the suffix is one number and compares exactly.
// A string compare would put "kubeletSysext1356" after "kubeletSysext13510".
func newestPatchDigest(packages map[string]string, prefix string) string {
	best, bestPatch := "", -1
	for name, patch := range versionedImages(packages, prefix) {
		if patch > bestPatch {
			best, bestPatch = packages[name], patch
		}
	}
	return best
}

// sandboxImage pins the pause image to the cluster's own registry: the CRD's
// registry.k8s.io default is unreachable from the node. It takes the registry
// the document carries, so the reference and spec.registry cannot disagree.
func sandboxImage(registry *registrySpec, images map[string]any) (string, error) {
	common, err := digestGroup(images, commonDigestsKey)
	if err != nil {
		return "", err
	}

	digest := common[pauseImageName]
	if digest == "" {
		return "", fmt.Errorf("the installer image carries no %q.%q digest", commonDigestsKey, pauseImageName)
	}

	// One repository for every Deckhouse image: the group a digest is filed under
	// is a map key, not a path segment, and appending it yields a 404 that
	// surfaces as a pod sandbox nobody can create.
	return registry.Address + registry.Path + "@" + digest, nil
}

func digestGroup(images map[string]any, key string) (map[string]string, error) {
	raw, ok := images[key].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("the installer image carries no %q image digests", key)
	}

	group := make(map[string]string, len(raw))
	for name, digest := range raw {
		if value, ok := digest.(string); ok {
			group[name] = value
		}
	}
	return group, nil
}
