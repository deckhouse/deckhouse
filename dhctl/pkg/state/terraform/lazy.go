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

package terraform

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type StateLoader interface {
	PopulateMetaConfig() (*config.MetaConfig, error)
	PopulateClusterState() ([]byte, map[string]state.NodeGroupTerraformState, error)
}

type LazyTerraStateLoader struct {
	stateLoader StateLoader

	lock            sync.Mutex
	clusterState    []byte
	nodesTerraState map[string]state.NodeGroupTerraformState
	metaConfig      *config.MetaConfig
}

func NewLazyTerraStateLoader(stateLoader StateLoader) *LazyTerraStateLoader {
	return &LazyTerraStateLoader{
		stateLoader: stateLoader,
	}
}

func (l *LazyTerraStateLoader) PopulateMetaConfig() (*config.MetaConfig, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.metaConfig == nil {
		var err error
		l.metaConfig, err = l.stateLoader.PopulateMetaConfig()
		if err != nil {
			l.metaConfig = nil
			return nil, err
		}
	}

	return l.metaConfig, nil
}

func (l *LazyTerraStateLoader) PopulateClusterState() ([]byte, map[string]state.NodeGroupTerraformState, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	var err error

	if l.nodesTerraState == nil || l.clusterState == nil {
		l.clusterState, l.nodesTerraState, err = l.stateLoader.PopulateClusterState()
		if err != nil {
			l.nodesTerraState = nil
			l.clusterState = nil
			return nil, nil, err
		}
	}

	return l.clusterState, l.nodesTerraState, nil
}
