// Copyright 2025 Flant JSC
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

package deckhouse

import (
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	resourcesDestroyedKey   = "resources-were-deleted"
	commanderUUIDCheckedKey = "commander-uuid-checked"
)

type State struct {
	cache state.Cache
}

func NewState(stateCache state.Cache) *State {
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

func (s *State) CommanderUUID() (string, error) {
	inCache, err := s.cache.InCache(commanderUUIDCheckedKey)
	if err != nil {
		return "", err
	}
	if !inCache {
		return "", nil
	}

	uuidInCache, err := s.cache.Load(commanderUUIDCheckedKey)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(uuidInCache)), nil
}

func (s *State) SetCommanderUUID(uuid string) error {
	return s.cache.Save(commanderUUIDCheckedKey, []byte(uuid))
}
