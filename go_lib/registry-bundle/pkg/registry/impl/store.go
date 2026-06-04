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

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

var _ registry.Registry = (*StoreRegistry)(nil)

// StoreResolver resolves a store.Store by repository name.
type StoreResolver interface {
	StoreByRepo(repo string, fn func(ok bool, store store.Store))
	SortedRepos() []string
}

// StoreRegistry implements registry.Registry by delegating per-repo operations
// to stores resolved via a StoreResolver.
type StoreRegistry struct {
	resolver StoreResolver
}

func NewStoreRegistry(resolver StoreResolver) (registry.Registry, error) {
	if resolver == nil {
		return nil, fmt.Errorf("store resolver is nil")
	}
	return &StoreRegistry{resolver: resolver}, nil
}

func (s *StoreRegistry) Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)

	s.resolver.StoreByRepo(repo, func(ok bool, st store.Store) {
		if !ok {
			err = fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
			return
		}
		rc, err = st.Fetch(ctx, dgst)
	})
	return rc, err
}

func (s *StoreRegistry) Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error) {
	var (
		exists bool
		size   int64
		err    error
	)

	s.resolver.StoreByRepo(repo, func(ok bool, st store.Store) {
		if !ok {
			err = fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
			return
		}
		exists, size, err = st.Exists(ctx, dgst)
	})
	return exists, size, err
}

func (s *StoreRegistry) Resolve(ctx context.Context, repo string, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	var (
		desc types.ShortDescriptor
		rc   io.ReadCloser
		err  error
	)

	s.resolver.StoreByRepo(repo, func(ok bool, st store.Store) {
		if !ok {
			err = fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
			return
		}
		desc, rc, err = st.Resolve(ctx, reference)
	})
	return desc, rc, err
}

func (s *StoreRegistry) Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error) {
	var (
		descs []ocispec.Descriptor
		err   error
	)

	s.resolver.StoreByRepo(repo, func(ok bool, st store.Store) {
		if !ok {
			err = fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
			return
		}
		descs, err = st.Predecessors(ctx, dgst)
	})
	return descs, err
}

func (s *StoreRegistry) SortedTags(ctx context.Context, repo string, last string) ([]string, error) {
	var (
		tags []string
		err  error
	)

	s.resolver.StoreByRepo(repo, func(ok bool, st store.Store) {
		if !ok {
			err = fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
			return
		}
		tags, err = st.SortedTags(ctx, last)
	})
	return tags, err
}

func (s *StoreRegistry) SortedRepos() []string {
	return s.resolver.SortedRepos()
}
