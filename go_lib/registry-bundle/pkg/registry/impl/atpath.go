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

package impl

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// AtPathRegistry wraps [registry.Registry] so it is mounted under a virtual root path.
// Requests for "root/foo" are forwarded to the inner registry as "foo"; "root" maps to "".
// [AtPathRegistry.SortedRepos] prefixes inner names with rootRepo when non-empty.
var _ registry.Registry = (*AtPathRegistry)(nil)

type AtPathRegistry struct {
	rootRepo string
	reg      registry.Registry
}

// NewAtPathRegistry returns a [registry.Registry] that serves inner under the virtual root path rootRepo.
// Use rootRepo "" to pass repository names through unchanged.
func NewAtPathRegistry(rootRepo string, reg registry.Registry) (registry.Registry, error) {
	if reg == nil {
		return nil, fmt.Errorf("inner registry is nil")
	}

	return &AtPathRegistry{rootRepo: rootRepo, reg: reg}, nil
}

func (a *AtPathRegistry) Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	sub, ok := a.stripRootRepo(repo)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}
	return a.reg.Fetch(ctx, sub, dgst)
}

func (a *AtPathRegistry) Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error) {
	sub, ok := a.stripRootRepo(repo)
	if !ok {
		return false, 0, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}
	return a.reg.Exists(ctx, sub, dgst)
}

func (a *AtPathRegistry) Resolve(ctx context.Context, repo string, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	sub, ok := a.stripRootRepo(repo)
	if !ok {
		return types.ShortDescriptor{}, nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}
	return a.reg.Resolve(ctx, sub, reference)
}

func (a *AtPathRegistry) Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error) {
	sub, ok := a.stripRootRepo(repo)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}
	return a.reg.Predecessors(ctx, sub, dgst)
}

func (a *AtPathRegistry) SortedTags(ctx context.Context, repo string, last string) ([]string, error) {
	sub, ok := a.stripRootRepo(repo)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}
	return a.reg.SortedTags(ctx, sub, last)
}

func (a *AtPathRegistry) SortedRepos() []string {
	names := a.reg.SortedRepos()
	if a.rootRepo == "" {
		return names
	}

	ret := make([]string, 0, len(names))
	for _, n := range names {
		if n == "" {
			ret = append(ret, a.rootRepo)
		} else {
			ret = append(ret, path.Join(a.rootRepo, n))
		}
	}
	return ret
}

func (a *AtPathRegistry) stripRootRepo(repo string) (string, bool) {
	if a.rootRepo == "" {
		return repo, true
	}

	if repo == a.rootRepo {
		return "", true
	}

	prefix := a.rootRepo + "/"
	if strings.HasPrefix(repo, prefix) {
		return repo[len(prefix):], true
	}

	return "", false
}
