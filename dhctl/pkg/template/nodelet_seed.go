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

package template

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/nodeletseed"
)

// FirstMasterBashibleData is what the first master's scripts are rendered from: the
// bundle data plus the document its node agent starts from.
func FirstMasterBashibleData(ctx context.Context, metaConfig *config.MetaConfig, nodeIP, candiDir string) (map[string]any, error) {
	data, err := metaConfig.ConfigForBashibleBundleTemplate(ctx, nodeIP)
	if err != nil {
		return nil, fmt.Errorf("collect the bundle config: %w", err)
	}
	images, ok := data["images"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("read images from the bundle config: unexpected type %T", data["images"])
	}
	proxyEndpoints, ok := data["clusterMasterRPPAddresses"].([]string)
	if !ok {
		return nil, fmt.Errorf("read clusterMasterRPPAddresses from the bundle config: unexpected type %T", data["clusterMasterRPPAddresses"])
	}
	seed, err := nodeletseed.FirstMaster(ctx, candiDir, images, proxyEndpoints)
	if err != nil {
		return nil, fmt.Errorf("build the node agent seed: %w", err)
	}
	data["nodeletSeed"] = seed
	return data, nil
}
