// Copyright 2026 Flant JSC
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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	BastionHostCacheKey = "bastion-host"
)

func SaveBastionHostToCache(ctx context.Context, stateCache state.Cache, host string) error {
	if err := stateCache.Save(ctx, BastionHostCacheKey, []byte(host)); err != nil {
		return fmt.Errorf("failed to save bastion host to cache: %w", err)
	}

	return nil
}

func GetBastionHostFromCache(ctx context.Context, stateCache state.Cache) (string, error) {
	exists, err := stateCache.InCache(ctx, BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", nil
	}

	host, err := stateCache.Load(ctx, BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	return string(host), nil
}

func BootstrapGetNodesFromCache(
	ctx context.Context,
	metaConfig *config.MetaConfig,
	stateCache state.Cache,
) (map[string]map[int]string, error) {
	nodeGroupRegex := fmt.Sprintf("^%s-(.*)-([0-9]+)\\.tfstate$", metaConfig.ClusterPrefix)
	groupsReg, _ := regexp.Compile(nodeGroupRegex)

	nodesFromCache := make(map[string]map[int]string)

	err := stateCache.Iterate(ctx, func(name string, content []byte) error {
		switch {
		case strings.HasSuffix(name, ".backup"):
			fallthrough
		case strings.HasPrefix(name, string(infrastructure.BaseInfraStep)):
			fallthrough
		case strings.HasPrefix(name, "uuid"):
			fallthrough
		case !groupsReg.MatchString(name):
			return nil
		}

		nodeGroupNameAndNodeIndex := groupsReg.FindStringSubmatch(name)

		nodeGroupName := nodeGroupNameAndNodeIndex[1]
		rawIndex := nodeGroupNameAndNodeIndex[2]

		index, convErr := strconv.Atoi(rawIndex)
		if convErr != nil {
			return fmt.Errorf("can't convert %q to integer: %v", rawIndex, convErr)
		}

		if _, ok := nodesFromCache[nodeGroupName]; !ok {
			nodesFromCache[nodeGroupName] = make(map[int]string)
		}

		nodesFromCache[nodeGroupName][index] = strings.TrimSuffix(name, ".tfstate")
		return nil
	})
	return nodesFromCache, err
}
