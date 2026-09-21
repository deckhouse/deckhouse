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

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
	if len(versions) == 0 {
		return nil, fmt.Errorf("no release version is known, so the set is unknown")
	}

	puller, err := remote.NewPuller(source.Options...)
	if err != nil {
		return nil, fmt.Errorf("building the client: %w", err)
	}

	references, err := Release{Versions: versions, Modules: modules}.Discover(ctx, source, puller)
	if err != nil {
		return nil, err
	}

	declared := make(map[string]struct{}, len(references))
	for _, reference := range references {
		if digest, ok := reference.(name.Digest); ok {
			declared[digest.DigestStr()] = struct{}{}
			continue
		}

		// A tag — the release image and its installer. What counts is whatever it points at now.
		descriptor, err := puller.Head(ctx, reference)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", reference, err)
		}
		declared[descriptor.Digest.String()] = struct{}{}
	}

	return declared, nil
}

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
