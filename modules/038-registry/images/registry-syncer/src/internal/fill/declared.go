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

package fill

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

// DeclaredDigests is every manifest the given release versions declare, as digests.
//
// The one definition of "the set", for everything that has an opinion about how much a replica holds.
// There used to be two, and a cluster showed what that costs: a replica reported 333 after a pass that
// copied the set and 348 after a pass that counted its disk — the second number including images the
// pass-through cache had settled, which belong to no release. `full` followed whichever pass ran last,
// so it flapped; eligibility follows `full`, so the lease moved; and a fill restarts on every move. The
// lease travelled between three replicas every twenty seconds and none of them ever finished.
func DeclaredDigests(
	ctx context.Context, source Registry, versions []string, modules []ModuleRef,
) (map[string]struct{}, error) {
	declared, err := DeclareSet(ctx, source, versions, modules)
	if err != nil {
		return nil, err
	}
	return declared.Digests, nil
}

// Declared is the set as DeclaredDigests reads it, with what declares each member.
type Declared struct {
	// Digests is the set — see DeclaredDigests.
	Digests map[string]struct{}

	// Owners are what declares each digest of the set: the releases, modules and images, merged so
	// that one image of two releases is one owner with two versions. Read by the listing of the
	// cache's images and by nothing that decides anything.
	Owners map[string][]registryimagev1alpha1.ImageOwner
}

// DeclareSet is DeclaredDigests with the owner of every digest. One reading for both, so that the
// listing of images by component cannot disagree with the set the replica is judged complete on.
func DeclareSet(
	ctx context.Context, source Registry, versions []string, modules []ModuleRef,
) (Declared, error) {
	if len(versions) == 0 {
		return Declared{}, fmt.Errorf("no release version is known, so the set is unknown")
	}

	puller, err := remote.NewPuller(source.Options...)
	if err != nil {
		return Declared{}, fmt.Errorf("building the client: %w", err)
	}

	byReference := map[string][]registryimagev1alpha1.ImageOwner{}
	release := Release{Versions: versions, Modules: modules,
		Attribute: func(reference name.Reference, owner registryimagev1alpha1.ImageOwner) {
			byReference[reference.String()] = append(byReference[reference.String()], owner)
		},
	}
	references, err := release.Discover(ctx, source, puller)
	if err != nil {
		return Declared{}, err
	}

	declared := Declared{
		Digests: make(map[string]struct{}, len(references)),
		Owners:  make(map[string][]registryimagev1alpha1.ImageOwner, len(references)),
	}
	own := func(key string, reference name.Reference) {
		declared.Digests[key] = struct{}{}
		declared.Owners[key] = MergeOwners(declared.Owners[key], byReference[reference.String()]...)
	}
	for _, reference := range references {
		if digest, ok := reference.(name.Digest); ok {
			own(digest.DigestStr(), reference)
			continue
		}

		// A tag — the release image, a module package. What counts is whatever it points at now.
		descriptor, err := puller.Head(ctx, reference)
		if absentAtSource(err) {
			// Not in this store yet. It is still part of the set, as something this store does not
			// hold: a key no manifest on disk can match, so the count stays below the set until it
			// arrives. Failing instead left the replica with no idea what the cluster needs.
			own(missingPrefix+reference.String(), reference)
			continue
		}
		if err != nil {
			return Declared{}, fmt.Errorf("resolving %s: %w", reference, err)
		}
		own(descriptor.Digest.String(), reference)
	}

	return declared, nil
}

// MergeOwners adds owners to into, merging an owner that is already there — the same kind, module,
// image and source — by the union of their versions. The result is sorted.
func MergeOwners(
	into []registryimagev1alpha1.ImageOwner, owners ...registryimagev1alpha1.ImageOwner,
) []registryimagev1alpha1.ImageOwner {
	for _, owner := range owners {
		merged := false
		for i := range into {
			existing := &into[i]
			if existing.Kind != owner.Kind || existing.Module != owner.Module ||
				existing.Image != owner.Image || existing.Source != owner.Source {
				continue
			}
			for _, version := range owner.Versions {
				if !slices.Contains(existing.Versions, version) {
					existing.Versions = append(existing.Versions, version)
				}
			}
			sort.Strings(existing.Versions)
			merged = true
			break
		}
		if !merged {
			owner.Versions = slices.Clone(owner.Versions)
			sort.Strings(owner.Versions)
			into = append(into, owner)
		}
	}
	sort.SliceStable(into, func(i, j int) bool {
		a, b := into[i], into[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Image != b.Image {
			return a.Image < b.Image
		}
		return a.Source < b.Source
	})
	return into
}

// missingPrefix marks a member of the declared set that could not be resolved to a digest, because
// the store does not hold it yet. Never a digest, so never held.
const missingPrefix = "missing:"

// CountDeclaredHeld is how much of the set this replica holds, counted on its own disk.
//
// Restricted to the declared set on purpose. Counting everything on disk answers a different question —
// "how much is in this store" — and mixing the two answers into one field is what made completeness
// flap: a store legitimately holds more than any release declares, because a pass-through cache settles
// whatever the cluster pulls.
func CountDeclaredHeld(root, scope string, declared map[string]struct{}) (int32, error) {
	if len(declared) == 0 {
		return 0, nil
	}

	held, err := HeldManifests(root, scope)
	if err != nil {
		return 0, err
	}

	found := make(map[string]struct{}, len(held))
	for _, manifest := range held {
		if _, ours := declared[manifest.Digest]; ours {
			found[manifest.Digest] = struct{}{}
		}
	}
	return int32(len(found)), nil
}
