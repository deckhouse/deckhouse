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

// Discoverer decides what a copy consists of.
//
// An explicit choice at every call site, because the two callers need different answers and
// there is no safe default. A copy between our own replicas can ask the source what it holds;
// a copy from somebody else's registry cannot — and getting that wrong is not a failure that
// announces itself, it is a cache that quietly holds the wrong set.
type Discoverer interface {
	// Discover lists what to copy out of the source.
	Discover(ctx context.Context, source Registry, puller *remote.Puller) ([]name.Reference, error)
}

// Catalogue asks the source registry what it holds.
//
// For copies where the source is one of our own replicas: the registry is ours, so listing it
// is both permitted and exactly right — a follower wants whatever the leader has.
//
// Not usable against an upstream. Listing a registry's catalogue is a privilege of its own,
// and credentials scoped to pulling a repository — which is all a Deckhouse license grants —
// are refused for it. What that looked like in a cluster was a fill retrying every thirty
// seconds forever, with the reason recorded only in a status field.
type Catalogue struct{}

func (Catalogue) Discover(
	ctx context.Context, source Registry, puller *remote.Puller,
) ([]name.Reference, error) {
	registry, err := parseRegistry(source)
	if err != nil {
		return nil, err
	}

	catalogger, err := puller.Catalogger(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("listing the source repositories: %w", err)
	}

	var references []name.Reference
	for catalogger.HasNext() {
		page, err := catalogger.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading the next page of source repositories: %w", err)
		}

		for _, repository := range page.Repos {
			if !inScope(source.Repository, repository) {
				continue
			}

			repositoryName := registry.Repo(repository)
			lister, err := puller.Lister(ctx, repositoryName)
			if err != nil {
				return nil, fmt.Errorf("listing the tags of %s: %w", repositoryName, err)
			}

			for lister.HasNext() {
				tags, err := lister.Next(ctx)
				if err != nil {
					return nil, fmt.Errorf("reading the next page of tags of %s: %w", repositoryName, err)
				}
				for _, tag := range tags.Tags {
					references = append(references, repositoryName.Tag(tag))
				}
			}
		}
	}

	return references, nil
}

// inScope keeps a copy to the configured repository prefix. Without it a shared registry
// would have everything on it pulled into a cache sized for Deckhouse.
func inScope(prefix, repository string) bool {
	if prefix == "" {
		return true
	}
	return repository == prefix ||
		len(repository) > len(prefix) && repository[:len(prefix)+1] == prefix+"/"
}

func parseRegistry(registry Registry) (name.Registry, error) {
	var options []name.Option
	if registry.Insecure {
		options = append(options, name.Insecure)
	}

	parsed, err := name.NewRegistry(registry.Address, options...)
	if err != nil {
		return name.Registry{}, fmt.Errorf("parsing the registry address %q: %w", registry.Address, err)
	}
	return parsed, nil
}
