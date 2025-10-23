// Copyright 2021 Flant JSC
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

package destroy

import "github.com/deckhouse/deckhouse/dhctl/pkg/state"

const (
	resourcesDestroyedKey = "resources-were-deleted"
	convergeLocked        = "converge-locked"
)

type State struct {
	cache state.Cache
}

func NewDestroyState(stateCache state.Cache) *State {
	return &State{
		cache: stateCache,
	}
}

func (s *State) IsResourcesDestroyed() (bool, error) {
	return s.cache.InCache(resourcesDestroyedKey)
}

func (s *State) SetResourcesDestroyed() error {
	return s.cache.Save(resourcesDestroyedKey, []byte("yes"))
}

func (s *State) IsConvergeLocked() (bool, error) {
	return s.cache.InCache(convergeLocked)
}

func (s *State) SetConvergeLocked() error {
	return s.cache.Save(convergeLocked, []byte("yes"))
}

func (s *State) Clean() {
	s.cache.Clean()
}
