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

package mocks

import (
	"context"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// MockRegistry implements Registry for testing.
type MockRegistry struct {
	ResolveFunc      func(ctx context.Context, repo, reference string) (types.ShortDescriptor, io.ReadCloser, error)
	FetchFunc        func(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error)
	ExistsFunc       func(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error)
	PredecessorsFunc func(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error)
	SortedTagsFunc   func(ctx context.Context, repo, last string) ([]string, error)
	SortedReposFunc  func() []string
}

func (m *MockRegistry) Resolve(ctx context.Context, repo, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	return m.ResolveFunc(ctx, repo, reference)
}
func (m *MockRegistry) Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	return m.FetchFunc(ctx, repo, dgst)
}
func (m *MockRegistry) Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error) {
	return m.ExistsFunc(ctx, repo, dgst)
}
func (m *MockRegistry) Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error) {
	return m.PredecessorsFunc(ctx, repo, dgst)
}
func (m *MockRegistry) SortedTags(ctx context.Context, repo, last string) ([]string, error) {
	return m.SortedTagsFunc(ctx, repo, last)
}
func (m *MockRegistry) SortedRepos() []string {
	return m.SortedReposFunc()
}

func NopRegistry() *MockRegistry {
	return &MockRegistry{
		ResolveFunc: func(_ context.Context, _, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
			panic("unexpected")
		},
		FetchFunc: func(_ context.Context, _ string, _ digest.Digest) (io.ReadCloser, error) {
			panic("unexpected")
		},
		ExistsFunc: func(_ context.Context, _ string, _ digest.Digest) (bool, int64, error) {
			panic("unexpected")
		},
		PredecessorsFunc: func(_ context.Context, _ string, _ digest.Digest) ([]ocispec.Descriptor, error) {
			panic("unexpected")
		},
		SortedTagsFunc: func(_ context.Context, _, _ string) ([]string, error) {
			panic("unexpected")
		},
		SortedReposFunc: func() []string {
			panic("unexpected")
		},
	}
}
