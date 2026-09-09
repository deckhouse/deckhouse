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

package gc

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// releaseGVK is the resource the cluster records its own versions in.
//
// Read as unstructured rather than through the typed API of the Deckhouse controller: this
// binary needs two strings out of it, and taking a dependency on that module's types to get
// them would tie a component of the data plane to the release machinery's Go API.
var releaseGVK = schema.GroupVersionKind{
	Group:   "deckhouse.io",
	Version: "v1alpha1",
	Kind:    "DeckhouseReleaseList",
}

// Phases a release can be in. Only these two matter here: one says what the cluster runs, the
// other what it ran before.
const (
	phaseDeployed   = "Deployed"
	phaseSuperseded = "Superseded"
)

// FromCluster reads what the cluster is running.
//
// The deployed release is the one the collector judges everything against, and the most
// recent superseded one is kept so that a rollback does not have to re-download the release
// it rolls back to.
//
// Returns an error rather than a partial answer when nothing is deployed. The collector needs
// that version to justify any deletion at all, and an empty answer would read as "keep
// nothing".
func FromCluster(ctx context.Context, c client.Client) (Releases, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(releaseGVK)

	if err := c.List(ctx, list); err != nil {
		return Releases{}, fmt.Errorf("listing the cluster's releases: %w", err)
	}

	var releases Releases
	for i := range list.Items {
		item := &list.Items[i]

		version, found, err := unstructured.NestedString(item.Object, "spec", "version")
		if err != nil || !found || version == "" {
			continue
		}
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")

		switch phase {
		case phaseDeployed:
			releases.Deployed = version
		case phaseSuperseded:
			// The most recent of them, which is not the same as the last one listed: the API
			// returns them in name order, and names are versions as strings — where v1.9.0
			// sorts after v1.10.0.
			if newer, err := isNewer(normalize(version), normalize(releases.Previous)); releases.Previous == "" || (err == nil && newer) {
				releases.Previous = version
			}
		}
	}

	if releases.Deployed == "" {
		// No release object says what the cluster runs, so the cluster itself is asked.
		//
		// A release object is not the only way a cluster can be running a version, and treating
		// it as one leaves whole clusters with neither a fill nor a collection: on a cluster
		// installed from a development or pull-request image no DeckhouseRelease is ever
		// deployed, and this function returned an error on every pass. Measured on such a
		// cluster: `RegistryStorage` in `phase: Failed` with `FillFailed`, all three replicas
		// reporting "no release is deployed", the leader's store holding nothing and the
		// followers holding nothing to replicate. The cache was empty by construction, on the
		// one axis — a cache with an upstream — the module exists to serve.
		//
		// What is read instead is the image the cluster is actually running, and that is the
		// same answer by a shorter route: the version is only ever used to find the installer
		// image that declares the image set, exactly as `d8 mirror pull` finds it, and the
		// running image names its own installer. Nothing about the set changes — the same
		// images, discovered from the same file in the same image.
		version, err := runningVersion(ctx, c)
		if err != nil {
			return releases, fmt.Errorf(
				"no release is deployed, and the running version could not be read either, "+
					"so there is nothing to judge the store against: %w", err)
		}
		releases.Deployed = version
	}
	return releases, nil
}

// deploymentGVK is how the running version is read, without depending on the release machinery.
var deploymentGVK = schema.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

// runningVersion returns the version tag of the Deckhouse image the cluster is running.
//
// The tag and not a digest, because what it is for is naming the installer image beside it —
// `install:<tag>` — whose digests file declares the image set. A deployment pinned to a digest
// alone therefore cannot answer, and says so rather than guessing: the set would be unknown, and
// an unknown set must not become the justification for deleting anything.
func runningVersion(ctx context.Context, c client.Client) (string, error) {
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(deploymentGVK)

	key := client.ObjectKey{Namespace: "d8-system", Name: "deckhouse"}
	if err := c.Get(ctx, key, deployment); err != nil {
		return "", fmt.Errorf("reading the deckhouse deployment: %w", err)
	}

	containers, found, err := unstructured.NestedSlice(
		deployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) == 0 {
		return "", fmt.Errorf("the deckhouse deployment declares no containers")
	}

	for _, entry := range containers {
		container, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(container, "name")
		if name != "deckhouse" {
			continue
		}

		image, _, _ := unstructured.NestedString(container, "image")
		tag, ok := imageTag(image)
		if !ok {
			return "", fmt.Errorf(
				"the running image %q names no tag, so the installer that declares its image set "+
					"cannot be found", image)
		}
		return tag, nil
	}

	return "", fmt.Errorf("the deckhouse deployment has no container named deckhouse")
}

// imageTag takes the tag out of an image reference.
//
// Written out because the two colons in a reference mean different things: the one in
// `registry:5001` is a port and the one after the last slash is the tag. A reference pinned by
// digest has no tag at all, and neither has one that relies on the implicit `latest` — both are
// refused rather than turned into something to look up.
func imageTag(image string) (string, bool) {
	if image == "" {
		return "", false
	}

	lastSlash := strings.LastIndex(image, "/")
	remainder := image[lastSlash+1:]

	if index := strings.Index(remainder, "@"); index >= 0 {
		// Digest-pinned; a tag may still precede the digest.
		remainder = remainder[:index]
	}

	colon := strings.LastIndex(remainder, ":")
	if colon <= 0 || colon == len(remainder)-1 {
		return "", false
	}
	return remainder[colon+1:], true
}

// moduleReleaseGVK is where the cluster records which modules it keeps and at which versions.
//
// Read unstructured for the same reason as the platform's releases: two strings are needed out of it,
// and taking a dependency on the release machinery's Go types to get them would tie a data-plane
// component to that API.
var moduleReleaseGVK = schema.GroupVersionKind{
	Group:   "deckhouse.io",
	Version: "v1alpha1",
	Kind:    "ModuleReleaseList",
}

// ModulesFromCluster reads which modules the cluster keeps, so that the images they consist of can be
// counted as part of what the cluster needs.
//
// Deployed only. A module release that is pending or superseded is not what the cluster runs, and
// including it would raise the bar for completeness by images no pod refers to — the mirror image of
// the failure this exists to prevent, and just as effective at making air-gap unreachable.
//
// An absent CRD is not an error but an empty answer: a cluster can legitimately have no modules beyond
// the platform's own, and on such a cluster nothing about the previous behaviour changes.
func ModulesFromCluster(ctx context.Context, c client.Client) ([]fill.ModuleRef, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(moduleReleaseGVK)

	if err := c.List(ctx, list); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing the modules the cluster keeps: %w", err)
	}

	var modules []fill.ModuleRef
	for i := range list.Items {
		item := &list.Items[i]

		if phase, _, _ := unstructured.NestedString(item.Object, "status", "phase"); phase != phaseDeployed {
			continue
		}

		name, _, err := unstructured.NestedString(item.Object, "spec", "moduleName")
		if err != nil || name == "" {
			continue
		}
		version, _, err := unstructured.NestedString(item.Object, "spec", "version")
		if err != nil || version == "" {
			continue
		}

		// Versions are recorded without the leading "v" in the object and with it in the registry,
		// which is where these are about to be looked for.
		modules = append(modules, fill.ModuleRef{Name: name, Version: registryTag(version)})
	}

	return modules, nil
}

// registryTag is the module version as the registry tags it.
func registryTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}
