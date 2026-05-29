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
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/set"
)

// FallbackRegistry wraps multiple [registry.Registry] values and tries them in order.
// The first registry that does not return [errs.ErrUnknownRepository] wins.
var _ registry.Registry = (*FallbackRegistry)(nil)

type FallbackRegistry struct {
	regs []registry.Registry
}

// NewFallbackRegistry returns a [registry.Registry] that tries each input in order,
// skipping registries that return [errs.ErrUnknownRepository]. Returns an error
// if any input is nil. Returns the single input directly when only one is provided.
func NewFallbackRegistry(regs ...registry.Registry) (registry.Registry, error) {
	if len(regs) == 0 {
		return nil, fmt.Errorf("no inner registries provided")
	}

	for i, reg := range regs {
		if reg == nil {
			return nil, fmt.Errorf("inner registry at index %d is nil", i)
		}
	}

	if len(regs) == 1 {
		return regs[0], nil
	}

	return &FallbackRegistry{regs: regs}, nil
}

func (f *FallbackRegistry) Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	for _, reg := range f.regs {
		reader, err := reg.Fetch(ctx, repo, dgst)

		if errors.Is(err, errs.ErrUnknownRepository) {
			continue
		}

		return reader, err
	}

	return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
}

func (f *FallbackRegistry) Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error) {
	for _, reg := range f.regs {
		exist, size, err := reg.Exists(ctx, repo, dgst)

		if errors.Is(err, errs.ErrUnknownRepository) {
			continue
		}

		return exist, size, err
	}

	return false, 0, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
}

func (f *FallbackRegistry) Resolve(ctx context.Context, repo string, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	for _, reg := range f.regs {
		desc, reader, err := reg.Resolve(ctx, repo, reference)

		if errors.Is(err, errs.ErrUnknownRepository) {
			continue
		}

		return desc, reader, err
	}

	return types.ShortDescriptor{}, nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
}

func (f *FallbackRegistry) Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error) {
	for _, reg := range f.regs {
		descs, err := reg.Predecessors(ctx, repo, dgst)

		if errors.Is(err, errs.ErrUnknownRepository) {
			continue
		}

		return descs, err
	}

	return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
}

func (f *FallbackRegistry) SortedTags(ctx context.Context, repo string, last string) ([]string, error) {
	for _, reg := range f.regs {
		tags, err := reg.SortedTags(ctx, repo, last)

		if errors.Is(err, errs.ErrUnknownRepository) {
			continue
		}

		return tags, err
	}

	return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
}

func (f *FallbackRegistry) SortedRepos() []string {
	all := set.New[string]()

	for _, reg := range f.regs {
		all.Add(reg.SortedRepos()...)
	}

	sorted := all.Values()
	slices.Sort(sorted)
	return sorted
}
