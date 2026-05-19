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

package static

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	nodeUserKey       = "node-user"
	nodeUserExistsKey = "node-user-exists"
)

var (
	errNotFoundCredentials = errors.New("Not found node user credentials")
)

type State struct {
	cache state.Cache
}

func NewDestroyState(stateCache state.Cache) *State {
	return &State{
		cache: stateCache,
	}
}

func (s *State) SaveNodeUser(ctx context.Context, credentials *NodesWithCredentials) error {
	if err := s.cache.SaveStruct(ctx, nodeUserKey, credentials); err != nil {
		return fmt.Errorf("Cannot save node user credentials for static destroyer: %w", err)
	}

	return nil
}

func (s *State) NodeUser(ctx context.Context) (*NodesWithCredentials, error) {
	exists, err := s.cache.InCache(ctx, nodeUserKey)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errNotFoundCredentials
	}

	creds := NodesWithCredentials{}

	if err := s.cache.LoadStruct(ctx, nodeUserKey, &creds); err != nil {
		return nil, fmt.Errorf("Cannot load node user credentials for static destroyer: %w", err)
	}

	return &creds, nil
}

func (s *State) SetNodeUserExists(ctx context.Context) error {
	if err := s.cache.Save(ctx, nodeUserExistsKey, []byte("yes")); err != nil {
		return fmt.Errorf("Cannot save node user exists flag for static destroyer: %w", err)
	}

	return nil
}

func (s *State) IsNodeUserExists(ctx context.Context) bool {
	if exists, err := s.cache.InCache(ctx, nodeUserExistsKey); err != nil || !exists {
		return false
	}

	return true
}
