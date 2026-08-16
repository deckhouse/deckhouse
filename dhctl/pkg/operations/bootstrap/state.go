// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TODO move all states from operations/bootstrap to here

package bootstrap

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	PostBootstrapResultCacheKey      = "post-bootstrap-result"
	ManifestCreatedInClusterCacheKey = "tf-state-and-manifests-in-cluster"
)

type State struct {
	cache state.Cache
}

func NewBootstrapState(stateCache state.Cache) *State {
	return &State{
		cache: stateCache,
	}
}

func (s *State) SavePostBootstrapScriptResult(ctx context.Context, result string) error {
	return s.cache.Save(ctx, PostBootstrapResultCacheKey, []byte(result))
}

func (s *State) SaveManifestsCreated(ctx context.Context) error {
	return s.cache.Save(ctx, ManifestCreatedInClusterCacheKey, []byte("yes"))
}

func (s *State) IsManifestsCreated(ctx context.Context) (bool, error) {
	return s.cache.InCache(ctx, ManifestCreatedInClusterCacheKey)
}

func (s *State) PostBootstrapScriptResult(ctx context.Context) ([]byte, error) {
	return s.cache.Load(ctx, PostBootstrapResultCacheKey)
}

func (s *State) Clean(ctx context.Context) {
	s.cache.Clean(ctx)
}

func (s *State) Save(ctx context.Context, key string, data []byte) error {
	return s.cache.Save(ctx, key, data)
}

func (s *State) InCache(ctx context.Context, key string) (bool, error) {
	return s.cache.InCache(ctx, key)
}
