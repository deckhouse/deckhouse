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
	"context"
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

func (s *State) IsResourcesDestroyed(ctx context.Context) (bool, error) {
	return s.cache.InCache(ctx, resourcesDestroyedKey)
}

func (s *State) SetResourcesDestroyed(ctx context.Context) error {
	return s.cache.Save(ctx, resourcesDestroyedKey, []byte("yes"))
}

func (s *State) CommanderUUID(ctx context.Context) (string, error) {
	inCache, err := s.cache.InCache(ctx, commanderUUIDCheckedKey)
	if err != nil {
		return "", err
	}
	if !inCache {
		return "", nil
	}

	uuidInCache, err := s.cache.Load(ctx, commanderUUIDCheckedKey)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(uuidInCache)), nil
}

func (s *State) SetCommanderUUID(ctx context.Context, uuid string) error {
	return s.cache.Save(ctx, commanderUUIDCheckedKey, []byte(uuid))
}
