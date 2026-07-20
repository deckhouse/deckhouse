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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	PostBootstrapResultCacheKey      = "post-bootstrap-result"
	ManifestCreatedInClusterCacheKey = "tf-state-and-manifests-in-cluster"
	BashibleStepsStatusCacheKey      = "bashible-bundle-steps-status"
	RegistryPKICacheKey              = "registry-pki"
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

// SaveBashibleStepsStatus persists the set of bashible bundle steps that have
// already completed successfully (name -> content checksum), so a later
// dhctl run can resume the bootstrap bashible pipeline without re-running them.
func (s *State) SaveBashibleStepsStatus(ctx context.Context, statuses map[string]string) error {
	return s.cache.SaveStruct(ctx, BashibleStepsStatusCacheKey, statuses)
}

// BashibleStepsStatus loads the previously saved bashible bundle steps status.
// It returns an empty map, not an error, if nothing has been saved yet.
func (s *State) BashibleStepsStatus(ctx context.Context) (map[string]string, error) {
	inCache, err := s.cache.InCache(ctx, BashibleStepsStatusCacheKey)
	if err != nil {
		return nil, err
	}
	if !inCache {
		return map[string]string{}, nil
	}

	statuses := make(map[string]string)
	if err := s.cache.LoadStruct(ctx, BashibleStepsStatusCacheKey, &statuses); err != nil {
		return nil, err
	}

	return statuses, nil
}

// SaveRegistryPKI persists the generated registry CA/user PKI so subsequent
// bootstrap attempts for the same cluster reuse it instead of generating a
// new one every process invocation (which would push different credentials
// than an earlier, possibly interrupted, attempt already used).
func (s *State) SaveRegistryPKI(ctx context.Context, pki registry.PKI) error {
	return s.cache.SaveStruct(ctx, RegistryPKICacheKey, pki)
}

// RegistryPKI loads the previously saved registry PKI. ok is false, with no
// error, if nothing has been saved yet.
func (s *State) RegistryPKI(ctx context.Context) (pki registry.PKI, ok bool, err error) {
	inCache, err := s.cache.InCache(ctx, RegistryPKICacheKey)
	if err != nil {
		return registry.PKI{}, false, err
	}
	if !inCache {
		return registry.PKI{}, false, nil
	}

	if err := s.cache.LoadStruct(ctx, RegistryPKICacheKey, &pki); err != nil {
		return registry.PKI{}, false, err
	}

	return pki, true, nil
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
