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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func BootstrapTerraNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	terraNodeGroups []config.TerraNodeGroupSpec,
	infrastructureContext *infrastructure.Context,
	globalOptions *options.GlobalOptions,
) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create CloudPermanent NG", func(ctx context.Context) error {
		return operations.ParallelCreateNodeGroup(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext, globalOptions)
	})
}

func BootstrapAdditionalMasterNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	addressTracker map[string]string,
	infrastructureContext *infrastructure.Context,
	stateCache state.Cache,
	globalOptions *options.GlobalOptions,
) error {
	if metaConfig.MasterNodeGroupSpec.Replicas == 1 {
		dhlog.FromContext(ctx).DebugContext(ctx, "Skip bootstrap additional master nodes because replicas == 1")
		return nil
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Bootstrap additional master nodes", func(ctx context.Context) error {
		masterCloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, global.MasterNodeGroupName, global.ShowDeckhouseLogs)
		if err != nil {
			return err
		}

		for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
			outputs, err := operations.BootstrapAdditionalMasterNode(ctx, kubeCl, metaConfig, i, masterCloudConfig, infrastructureContext, globalOptions)
			if err != nil {
				return err
			}
			addressTracker[fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)] = outputs.MasterIPForSSH

			state.SaveMasterHostsToCache(ctx, stateCache, addressTracker)
		}

		return nil
	})
}
