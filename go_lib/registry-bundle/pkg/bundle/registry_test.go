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
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	storemocks "github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store/mocks"
)

func TestNewRegistry_ModuleListRegistry(t *testing.T) {
	tests := []struct {
		name             string
		repos            []string
		expectedRepoTags map[string][]string
	}{
		{
			name: "modules",
			repos: []string{
				"modules/a",
				"modules/b",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"a", "b"},
				PackagesRootPath:  nil, // ErrUnknownRepository
				D8PluginsRootPath: nil, // ErrUnknownRepository
			},
		},
		{
			name: "modules deep path",
			repos: []string{
				"modules/a/v1",
				"modules/b/v2",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"a", "b"},
				PackagesRootPath:  nil, // ErrUnknownRepository
				D8PluginsRootPath: nil, // ErrUnknownRepository
			},
		},
		{
			name: "packages",
			repos: []string{
				"packages/c",
				"packages/d",
			},
			expectedRepoTags: map[string][]string{
				PackagesRootPath:  {"c", "d"},
				ModulesRootPath:   nil, // ErrUnknownRepository
				D8PluginsRootPath: nil, // ErrUnknownRepository
			},
		},
		{
			name: "plugins",
			repos: []string{
				"deckhouse-cli/plugins/e",
				"deckhouse-cli/plugins/f",
			},
			expectedRepoTags: map[string][]string{
				D8PluginsRootPath: {"e", "f"},
				ModulesRootPath:   nil, // ErrUnknownRepository
				PackagesRootPath:  nil, // ErrUnknownRepository
			},
		},
		{
			name: "modules + packages",
			repos: []string{
				"modules/a",
				"packages/b",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"a"},
				PackagesRootPath:  {"b"},
				D8PluginsRootPath: nil, // ErrUnknownRepository
			},
		},
		{
			name: "modules + plugins",
			repos: []string{
				"modules/a",
				"deckhouse-cli/plugins/b",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"a"},
				D8PluginsRootPath: {"b"},
				PackagesRootPath:  nil, // ErrUnknownRepository
			},
		},
		{
			name: "packages + plugins",
			repos: []string{
				"packages/a",
				"deckhouse-cli/plugins/b",
			},
			expectedRepoTags: map[string][]string{
				PackagesRootPath:  {"a"},
				D8PluginsRootPath: {"b"},
				ModulesRootPath:   nil, // ErrUnknownRepository
			},
		},
		{
			name: "modules + packages + plugins",
			repos: []string{
				"modules/a",
				"packages/b",
				"deckhouse-cli/plugins/c",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"a"},
				PackagesRootPath:  {"b"},
				D8PluginsRootPath: {"c"},
			},
		},
		{
			name: "modules + packages + plugins + same tags",
			repos: []string{
				"modules/x",
				"packages/x",
				"deckhouse-cli/plugins/x",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   {"x"},
				PackagesRootPath:  {"x"},
				D8PluginsRootPath: {"x"},
			},
		},
		{
			name: "no one",
			repos: []string{
				"test/a",
				"test/b",
				"test/test/c",
			},
			expectedRepoTags: map[string][]string{
				ModulesRootPath:   nil, // ErrUnknownRepository
				PackagesRootPath:  nil, // ErrUnknownRepository
				D8PluginsRootPath: nil, // ErrUnknownRepository
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := storemocks.NopStore()
			st.SortedTagsFunc = func(_ context.Context, _ string) ([]string, error) {
				return nil, errs.ErrUnknownRepository
			}

			stores := make(map[string]store.Store, len(tt.repos))
			for _, repo := range tt.repos {
				stores[repo] = st
			}

			reg, err := NewRegistry("", &Bundle{repoStore: stores})
			if err != nil {
				t.Fatalf("NewRegistry: %v", err)
			}

			for expRepo, expTags := range tt.expectedRepoTags {
				tags, err := reg.SortedTags(t.Context(), expRepo, "")

				if expTags == nil {
					if !errors.Is(err, errs.ErrUnknownRepository) {
						t.Errorf("SortedTags(%q): expected ErrUnknownRepository, got err=%v tags=%v", expRepo, err, tags)
					}
					continue
				}

				if err != nil {
					t.Fatalf("SortedTags(%q): %v", expRepo, err)
				}

				if !slices.Equal(tags, expTags) {
					t.Errorf("SortedTags(%q) = %v, want %v", expRepo, tags, expTags)
				}
			}
		})
	}
}
