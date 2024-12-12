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

const PostBootstrapResultCacheKey = "post-bootstrap-result"
const PreflightBootstrapCloudResultCacheKey = "preflight-bootstrap-cloud-result"
const PreflightBootstrapPostCloudResultCacheKey = "preflight-bootstrap-post-cloud-result"
const PreflightBootstrapGlobalResultCacheKey = "preflight-bootstrap-global-result"
const PreflightBootstrapStaticResultCacheKey = "preflight-bootstrap-static-result"

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

func (s *State) PostBootstrapScriptResult() ([]byte, error) {
	return s.cache.Load(PostBootstrapResultCacheKey)
}

func (s *State) SetGlobalPreflightchecksWasRan() error {
	return s.cache.Save(PreflightBootstrapGlobalResultCacheKey, []byte("yes"))
}

func (s *State) GlobalPreflightchecksWasRan() (bool, error) {
	preflightcachefile, err := s.cache.InCache(PreflightBootstrapGlobalResultCacheKey)
	return preflightcachefile, err
}

func (s *State) SetCloudPreflightchecksWasRan() error {
	return s.cache.Save(PreflightBootstrapCloudResultCacheKey, []byte("yes"))
}

func (s *State) SetPostCloudPreflightchecksWasRan() error {
	return s.cache.Save(PreflightBootstrapPostCloudResultCacheKey, []byte("yes"))
}

func (s *State) CloudPreflightchecksWasRan() (bool, error) {
	preflightcachefile, err := s.cache.InCache(PreflightBootstrapCloudResultCacheKey)
	return preflightcachefile, err
}

func (s *State) PostCloudPreflightchecksWasRan() (bool, error) {
	preflightcachefile, err := s.cache.InCache(PreflightBootstrapPostCloudResultCacheKey)
	return preflightcachefile, err
}

func (s *State) SetStaticPreflightchecksWasRan() error {
	return s.cache.Save(PreflightBootstrapStaticResultCacheKey, []byte("yes"))
}

func (s *State) StaticPreflightchecksWasRan() (bool, error) {
	preflightcachefile, err := s.cache.InCache(PreflightBootstrapStaticResultCacheKey)
	return preflightcachefile, err
}

func (s *State) Clean() {
	s.cache.Clean()
}
