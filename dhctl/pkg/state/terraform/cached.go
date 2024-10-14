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
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type FileTerraStateLoader struct {
	stateCache state.Cache
	metaConfig *config.MetaConfig
}

func NewFileTerraStateLoader(stateCache state.Cache, metaConfig *config.MetaConfig) *FileTerraStateLoader {
	return &FileTerraStateLoader{
		stateCache: stateCache,
		metaConfig: metaConfig,
	}
}

func (s *FileTerraStateLoader) PopulateMetaConfig() (*config.MetaConfig, error) {
	return s.metaConfig, nil
}

func (s *FileTerraStateLoader) PopulateClusterState() ([]byte, map[string]state.NodeGroupTerraformState, error) {
	metaConfig, err := s.PopulateMetaConfig()
	if err != nil {
		return nil, nil, err
	}

	return getNodesFromCache(metaConfig, s.stateCache)
}

func getNodesFromCache(metaConfig *config.MetaConfig, stateCache state.Cache) ([]byte, map[string]state.NodeGroupTerraformState, error) {
	nodeGroupRegex := fmt.Sprintf("^%s-(.*)-([0-9]+)\\.tfstate$", metaConfig.ClusterPrefix)
	groupsReg, _ := regexp.Compile(nodeGroupRegex)

	nodesFromCache := make(map[string]state.NodeGroupTerraformState)

	var baseInfraState []byte

	err := stateCache.Iterate(func(name string, content []byte) error {
		switch {
		case strings.HasPrefix(name, "base-infrastructure"):
			baseInfraState = content
			return nil
		case strings.HasSuffix(name, ".backup"):
			fallthrough
		case strings.HasPrefix(name, "uuid"):
			fallthrough
		case !groupsReg.MatchString(name):
			return nil
		}

		nodeGroupNameAndNodeIndex := groupsReg.FindStringSubmatch(name)

		nodeGroupName := nodeGroupNameAndNodeIndex[1]

		if _, ok := nodesFromCache[nodeGroupName]; !ok {
			nodesFromCache[nodeGroupName] = state.NodeGroupTerraformState{
				State: map[string][]byte{},
			}
		}

		stateName := strings.TrimSuffix(name, ".tfstate")
		nodesFromCache[nodeGroupName].State[stateName] = content

		return nil
	})

	return baseInfraState, nodesFromCache, err
}

func DeleteNodeTerraformStateFromCache(nodeName string, stateCache state.Cache) error {
	keysToDelete := []string{
		fmt.Sprintf("%s.tfstate", nodeName),
		fmt.Sprintf("%s.tfstate.backup", nodeName),
	}

	for _, key := range keysToDelete {
		stateCache.Delete(key)
	}

	return nil
}
