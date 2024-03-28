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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type KubeClientGetter interface {
	GetKubeClient() (*client.KubernetesClient, error)
}

type KubeTerraStateLoader struct {
	kubeGetter KubeClientGetter
	stateCache state.Cache
}

func NewCachedTerraStateLoader(kubeGetter KubeClientGetter, stateCache state.Cache) *KubeTerraStateLoader {
	return &KubeTerraStateLoader{
		kubeGetter: kubeGetter,
		stateCache: stateCache,
	}
}

func (s *KubeTerraStateLoader) PopulateMetaConfig() (*config.MetaConfig, error) {
	var metaConfig *config.MetaConfig
	var err error

	confirmation := input.NewConfirmation().
		WithMessage("Do you want to continue with Cluster configuration from local cache?").
		WithYesByDefault()

	ok, err := s.stateCache.InCache("cluster-config")
	if err != nil {
		return nil, err
	}

	if ok && confirmation.Ask() {
		if err := s.stateCache.LoadStruct("cluster-config", &metaConfig); err != nil {
			return nil, err
		}
		return metaConfig, nil
	}

	kubeCl, err := s.kubeGetter.GetKubeClient()
	if err != nil {
		return nil, err
	}

	metaConfig, err = config.ParseConfigFromCluster(kubeCl)
	if err != nil {
		return nil, err
	}

	metaConfig.UUID, err = GetClusterUUID(kubeCl)
	if err != nil {
		return nil, err
	}

	if err := s.stateCache.SaveStruct("cluster-config", metaConfig); err != nil {
		return nil, err
	}

	return metaConfig, nil
}

func (s *KubeTerraStateLoader) PopulateClusterState() ([]byte, map[string]state.NodeGroupTerraformState, error) {
	clusterState, err := s.getClusterState()
	if err != nil {
		return nil, nil, err
	}

	nodesState, err := s.getNodesState()
	if err != nil {
		return nil, nil, err
	}

	return clusterState, nodesState, nil
}

func (s *KubeTerraStateLoader) getNodesState() (map[string]state.NodeGroupTerraformState, error) {
	var err error
	var kubeCl *client.KubernetesClient
	var nodesState map[string]state.NodeGroupTerraformState

	confirmation := input.NewConfirmation().
		WithMessage("Do you want to continue with Nodes state from local cache?").
		WithYesByDefault()

	ok, err := s.stateCache.InCache("nodes-state")
	if err != nil {
		return nil, err
	}

	if ok && confirmation.Ask() {
		if err := s.stateCache.LoadStruct("nodes-state", &nodesState); err != nil {
			return nil, err
		}
	} else {
		if kubeCl, err = s.kubeGetter.GetKubeClient(); err != nil {
			return nil, err
		}
		nodesState, err = GetNodesStateFromCluster(kubeCl)
		if err != nil {
			return nil, err
		}
		err := s.stateCache.SaveStruct("nodes-state", nodesState)
		if err != nil {
			return nil, err
		}
	}

	return nodesState, nil
}

func (s *KubeTerraStateLoader) getClusterState() ([]byte, error) {
	var kubeCl *client.KubernetesClient
	var err error
	var clusterState []byte

	confirmation := input.NewConfirmation().
		WithMessage("Do you want to continue with Cluster state from local cache?").
		WithYesByDefault()

	ok, err := s.stateCache.InCache("cluster-state")
	if err != nil {
		return nil, err
	}

	if ok && confirmation.Ask() {
		clusterState, err = s.stateCache.Load("cluster-state")
		if err != nil || len(clusterState) == 0 {
			return nil, fmt.Errorf("can't load cluster state from cache")
		}
	} else {
		if kubeCl, err = s.kubeGetter.GetKubeClient(); err != nil {
			return nil, err
		}
		clusterState, err = GetClusterStateFromCluster(kubeCl)
		if err != nil {
			return nil, err
		}
		if err := s.stateCache.Save("cluster-state", clusterState); err != nil {
			return nil, err
		}
	}

	return clusterState, nil
}
