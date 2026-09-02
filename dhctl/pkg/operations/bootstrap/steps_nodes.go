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

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
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

// BootstrapAdditionalMasterNodes creates every master past the first one.
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
		dhlog.FromContext(ctx).DebugContext(ctx, "Skipping additional master node bootstrap because replicas == 1")
		return nil
	}

	immutableMaster := immutable.IsImmutableMaster(ctx, metaConfig)

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Bootstrap additional master nodes", func(ctx context.Context) error {
		// The group's published cloud config is a bashible bundle, which an
		// immutable node cannot run: its payload is rendered here instead, and it
		// carries the node's own name — hence per node, below.
		masterCloudConfig := ""
		if !immutableMaster {
			var err error
			masterCloudConfig, err = entity.GetCloudConfig(ctx, kubernetes.NewSimpleKubeClientGetter(kubeCl), global.MasterNodeGroupName, global.ShowDeckhouseLogs)
			if err != nil {
				return err
			}
		}

		for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
			nodeName := fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)

			nodeCloudConfig := masterCloudConfig
			if immutableMaster {
				var err error
				nodeCloudConfig, err = buildImmutableJoinPayload(ctx, kubeCl, metaConfig, nodeName)
				if err != nil {
					return fmt.Errorf("build the payload of %s: %w", nodeName, err)
				}
			}

			outputs, err := operations.BootstrapAdditionalMasterNode(ctx, kubeCl, metaConfig, i, nodeCloudConfig, infrastructureContext, globalOptions)
			if err != nil {
				return err
			}

			// Converge builds its SSH session from this cache, and a host that
			// answers no sshd stalls it. The first master is kept out of the same
			// cache for the same reason.
			if immutableMaster {
				continue
			}
			addressTracker[nodeName] = outputs.MasterIPForSSH

			state.SaveMasterHostsToCache(ctx, stateCache, addressTracker)
		}

		return nil
	})
}
