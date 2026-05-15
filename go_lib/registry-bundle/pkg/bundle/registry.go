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

package bundle

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry/impl"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/set"
)

var _ impl.StoreResolver = (*storeResolverAdapter)(nil)

func NewRegistry(path string, bundle *Bundle) (registry.Registry, error) {
	if bundle == nil {
		return nil, fmt.Errorf("bundle is nil")
	}
	if err := bundle.validate(); err != nil {
		return nil, err
	}

	reg, err := impl.NewStoreRegistry(
		newStoreResolverAdapter(bundle),
	)
	if err != nil {
		return nil, fmt.Errorf("bundle store registry: %w", err)
	}

	listReg, err := newListRegistry(
		bundle,
		ModulesRootPath,   // /modules
		PackagesRootPath,  // /packages`
		D8PluginsRootPath, // /deckhouse-cli/plugins
	)

	if err != nil {
		return nil, fmt.Errorf("bundle list registry: %w", err)
	}

	if listReg != nil {
		reg, err = impl.NewFallbackRegistry(reg, listReg)
		if err != nil {
			return nil, fmt.Errorf("fallback registry: %w", err)
		}
	}

	reg, err = impl.NewAtPathRegistry(path, reg)
	if err != nil {
		return nil, fmt.Errorf("at path registry: %w", err)
	}
	return reg, nil
}

type storeResolverAdapter struct {
	bundle *Bundle
	repos  []string
}

func newStoreResolverAdapter(bundle *Bundle) *storeResolverAdapter {
	repos := make([]string, 0, len(bundle.repoStore))
	for repo := range bundle.repoStore {
		repos = append(repos, repo)
	}
	slices.Sort(repos)

	return &storeResolverAdapter{
		bundle: bundle,
		repos:  repos,
	}
}

func (sr *storeResolverAdapter) StoreByRepo(repo string, fn func(ok bool, st store.Store)) {
	st, ok := sr.bundle.repoStore[repo]
	fn(ok, st)
}

func (sr storeResolverAdapter) SortedRepos() []string {
	return sr.repos
}

func newListRegistry(bundle *Bundle, rootPaths ...string) (registry.Registry, error) {
	repos := make([]string, 0, len(bundle.repoStore))
	for repo := range bundle.repoStore {
		repos = append(repos, repo)
	}

	repoTags := map[string]set.Set[string]{}

	for _, root := range rootPaths {
		tags := subdirs(root, repos)
		if tags.Len() != 0 {
			repoTags[root] = tags
		}
	}

	if len(repoTags) == 0 {
		return nil, nil
	}

	reg, err := impl.NewEmptyImgRegistry(repoTags)
	if err != nil {
		return nil, fmt.Errorf("empty img registry: %w", err)
	}
	return reg, nil
}

func subdirs(rootPath string, paths []string) set.Set[string] {
	seen := set.New[string]()

	prefix := rootPath + "/"
	for _, p := range paths {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok || rest == "" {
			continue
		}
		seen.Add(strings.SplitN(rest, "/", 2)[0])
	}
	return seen
}
