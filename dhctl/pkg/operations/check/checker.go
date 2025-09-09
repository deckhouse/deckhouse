// Copyright 2023 Flant JSC
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

package check

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type Params struct {
	SSHClient     node.SSHClient
	StateCache    dhctlstate.Cache
	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	InfrastructureContext *infrastructure.Context

	KubeClient *client.KubernetesClient // optional
}

type Checker struct {
	*Params
}

func NewChecker(params *Params) *Checker {
	if !params.CommanderMode {
		panic("check operation currently supported only in commander mode")
	}

	// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
	// if params.CommanderUUID == uuid.Nil {
	//	panic("CommanderUUID required for check operation in commander mode!")
	// }

	return &Checker{
		Params: params,
	}
}

func (c *Checker) Check(ctx context.Context) (*CheckResult, error) {
	kubeCl, err := c.GetKubeClient(ctx)
	if err != nil {
		return nil, err
	}

	metaConfig, err := commander.ParseMetaConfig(c.StateCache, c.Params.CommanderModeParams)
	if c.InfrastructureContext == nil {
		c.InfrastructureContext = infrastructure.NewContextWithProvider(infrastructureprovider.ExecutorProvider(metaConfig))
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
	}

	res := &CheckResult{
		Status: CheckStatusInSync,
	}

	if c.CommanderMode {
		shouldUpdate, err := commander.CheckShouldUpdateCommanderUUID(ctx, kubeCl, c.CommanderUUID)
		if err != nil {
			return nil, fmt.Errorf("uuid consistency check failed: %w", err)
		}
		if shouldUpdate {
			res.Status = res.Status.CombineStatus(CheckStatusOutOfSync)
		}
	}

	hasTerraformState := false

	if metaConfig.ClusterType == config.CloudClusterType {
		resInfra, err := c.checkInfra(ctx, kubeCl, metaConfig, c.InfrastructureContext)
		if err != nil {
			return nil, fmt.Errorf("unable to check infra state: %w", err)
		}
		res.Status = res.Status.CombineStatus(resInfra.Status)

		if resInfra.Status == CheckStatusDestructiveOutOfSync {
			res.DestructiveChangeID, err = DestructiveChangeID(resInfra.Statistics)
			if err != nil {
				return nil, fmt.Errorf("unable to generate destructive change id: %w", err)
			}
		}

		hasTerraformState = resInfra.HasTerraformState
		res.StatusDetails.Statistics = *resInfra.Statistics
		res.StatusDetails.OpentofuMigrationStatus = resInfra.MigrationOpentofuStatus
	}

	configurationStatus, err := c.checkConfiguration(ctx, kubeCl, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to check configuration state: %w", err)
	}
	res.Status = res.Status.CombineStatus(configurationStatus)
	res.StatusDetails.ConfigurationStatus = configurationStatus
	res.HasTerraformState = hasTerraformState

	return res, nil
}

func (c *Checker) checkConfiguration(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (CheckStatus, error) {
	clusterConfigurationData, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get cluster config yaml: %w", err)
	}
	providerClusterConfigurationData, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get provider cluster config yaml: %w", err)
	}

	inClusterMetaConfig, err := entity.GetMetaConfig(ctx, kubeCl)
	if err != nil {
		return "", fmt.Errorf("unable to get in-cluster meta config: %w", err)
	}
	inClusterConfigurationData, err := inClusterMetaConfig.ClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get cluster config yaml: %w", err)
	}
	inClusterProviderClusterConfigurationData, err := inClusterMetaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get provider cluster config yaml: %w", err)
	}

	if inClusterMetaConfig.UUID == metaConfig.UUID && bytes.Equal(clusterConfigurationData, inClusterConfigurationData) && bytes.Equal(providerClusterConfigurationData, inClusterProviderClusterConfigurationData) {
		return CheckStatusInSync, nil
	}
	return CheckStatusOutOfSync, nil
}

type InfraResult struct {
	Status                  CheckStatus
	Statistics              *Statistics
	HasTerraformState       bool
	MigrationOpentofuStatus CheckStatus
}

func (c *Checker) checkInfra(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, infrastructureContext *infrastructure.Context) (*InfraResult, error) {
	stat, hasTerraformState, err := CheckState(
		ctx, kubeCl, metaConfig, infrastructureContext,
		CheckStateOptions{
			CommanderMode: c.CommanderMode,
			StateCache:    c.StateCache,
		},
	)
	if err != nil {
		return nil, err
	}

	checkStatus := CheckStatusInSync

	if stat != nil {
		for _, node := range stat.Node {
			checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(node.Status))
		}
		for _, nodeTempl := range stat.NodeTemplates {
			checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(nodeTempl.Status))
		}
		checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(stat.Cluster.Status))
	}

	migrateToTofuStatus := CheckStatusInSync

	if infrastructure.NeedToUseOpentofu(metaConfig) && hasTerraformState {
		checkStatus = checkStatus.CombineStatus(CheckStatusOutOfSync)
		migrateToTofuStatus = CheckStatusOutOfSync
	}

	return &InfraResult{
		Status:                  checkStatus,
		Statistics:              stat,
		HasTerraformState:       hasTerraformState,
		MigrationOpentofuStatus: migrateToTofuStatus,
	}, nil
}

func resolveStatisticsStatus(status string) CheckStatus {
	switch status {
	case OKStatus:
		return CheckStatusInSync
	case ChangedStatus:
		// NOTE: Regular out-of-sync state, which can be fixed by the converge run
		return CheckStatusOutOfSync
	case DestructiveStatus:
		// NOTE: Something will be destroyed by the converge run, such change should be approved
		return CheckStatusDestructiveOutOfSync
	case AbandonedStatus:
		// NOTE: Excess node — treat as destructive out-of-sync, because this node will be destroyed during converge run
		return CheckStatusDestructiveOutOfSync
	case AbsentStatus:
		// NOTE: Lost node — treat as out-of-sync for now
		return CheckStatusOutOfSync
	case ErrorStatus:
		// NOTE: Unknown error, probably can be healed by the retry
		return CheckStatusDestructiveOutOfSync
	}
	panic(fmt.Sprintf("unknown check infra status: %q", status))
}

func (c *Checker) GetKubeClient(ctx context.Context) (*client.KubernetesClient, error) {
	if c.KubeClient != nil {
		return c.KubeClient, nil
	}

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(c.SSHClient))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
	}
	return kubeCl, nil
}
