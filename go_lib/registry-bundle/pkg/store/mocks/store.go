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
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// MockStore implements Store for testing.
type MockStore struct {
	FetchFunc        func(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error)
	ExistsFunc       func(ctx context.Context, dgst digest.Digest) (bool, int64, error)
	ResolveFunc      func(ctx context.Context, reference string) (types.ShortDescriptor, io.ReadCloser, error)
	PredecessorsFunc func(ctx context.Context, dgst digest.Digest) ([]ociv1.Descriptor, error)
	SortedTagsFunc   func(ctx context.Context, last string) ([]string, error)
}

func (m *MockStore) Fetch(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	return m.FetchFunc(ctx, dgst)
}

func (m *MockStore) Exists(ctx context.Context, dgst digest.Digest) (bool, int64, error) {
	return m.ExistsFunc(ctx, dgst)
}

func (m *MockStore) Resolve(ctx context.Context, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	return m.ResolveFunc(ctx, reference)
}

func (m *MockStore) Predecessors(ctx context.Context, dgst digest.Digest) ([]ociv1.Descriptor, error) {
	return m.PredecessorsFunc(ctx, dgst)
}

func (m *MockStore) SortedTags(ctx context.Context, last string) ([]string, error) {
	return m.SortedTagsFunc(ctx, last)
}

// NopStore returns a MockStore that panics on any call.
func NopStore() *MockStore {
	return &MockStore{
		FetchFunc: func(_ context.Context, _ digest.Digest) (io.ReadCloser, error) {
			panic("unexpected call to Fetch")
		},
		ExistsFunc: func(_ context.Context, _ digest.Digest) (bool, int64, error) {
			panic("unexpected call to Exists")
		},
		ResolveFunc: func(_ context.Context, _ string) (types.ShortDescriptor, io.ReadCloser, error) {
			panic("unexpected call to Resolve")
		},
		PredecessorsFunc: func(_ context.Context, _ digest.Digest) ([]ociv1.Descriptor, error) {
			panic("unexpected call to Predecessors")
		},
		SortedTagsFunc: func(_ context.Context, _ string) ([]string, error) {
			panic("unexpected call to SortedTags")
		},
	}
}
