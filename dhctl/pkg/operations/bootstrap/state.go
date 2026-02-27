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

func (s *State) SavePostBootstrapScriptResult(result string) error {
	return s.cache.Save(PostBootstrapResultCacheKey, []byte(result))
}

func (s *State) SaveManifestsCreated() error {
	return s.cache.Save(ManifestCreatedInClusterCacheKey, []byte("yes"))
}

func (s *State) IsManifestsCreated() (bool, error) {
	return s.cache.InCache(ManifestCreatedInClusterCacheKey)
}

func (s *State) PostBootstrapScriptResult() ([]byte, error) {
	return s.cache.Load(PostBootstrapResultCacheKey)
}

func (s *State) Clean() {
	s.cache.Clean()
}

func (s *State) Save(key string, data []byte) error {
	return s.cache.Save(key, data)
}

func (s *State) InCache(key string) (bool, error) {
	return s.cache.InCache(key)
}
