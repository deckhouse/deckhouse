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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type Params struct {
	SSHClient     *ssh.Client
	StateCache    dhctlstate.Cache
	CommanderMode bool
	*commander.CommanderModeParams

	TerraformContext *terraform.TerraformContext

	KubeClient *client.KubernetesClient // optional
}

type Checker struct {
	*Params
}

func NewChecker(params *Params) *Checker {
	if !params.CommanderMode {
		panic("check operation currently supported only in commander mode")
	}

	return &Checker{
		Params: params,
	}
}

func (c *Checker) Check(ctx context.Context) (*CheckResult, error) {
	kubeCl, err := c.kubeClient()
	if err != nil {
		return nil, err
	}

	metaConfig, err := commander.ParseMetaConfig(c.StateCache, c.Params.CommanderModeParams)
	if err != nil {
		return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
	}

	res := &CheckResult{
		Status: CheckStatusInSync,
	}

	if metaConfig.ClusterType == config.CloudClusterType {
		status, statusDetails, err := c.checkInfra(ctx, kubeCl, metaConfig, c.TerraformContext)
		if err != nil {
			return nil, fmt.Errorf("unable to check infra state: %w", err)
		}
		res.Status = res.Status.CombineStatus(status)

		if status == CheckStatusDestructiveOutOfSync {
			res.DestructiveChangeID, err = converge.DestructiveChangeID(statusDetails)
			if err != nil {
				return nil, fmt.Errorf("unable to generate destructive change id: %w", err)
			}
		}

		res.StatusDetails.Statistics = *statusDetails
	}

	configurationStatus, err := c.checkConfiguration(ctx, kubeCl, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to check configuration state: %w", err)
	}
	res.Status = res.Status.CombineStatus(configurationStatus)
	res.StatusDetails.ConfigurationStatus = configurationStatus

	return res, nil
}

func (c *Checker) checkConfiguration(_ context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (CheckStatus, error) {
	clusterConfigurationData, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get cluster config yaml: %w", err)
	}
	providerClusterConfigurationData, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return "", fmt.Errorf("unable to get provider cluster config yaml: %w", err)
	}

	inClusterMetaConfig, err := converge.GetMetaConfig(kubeCl)
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

func (c *Checker) checkInfra(_ context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraformContext *terraform.TerraformContext) (CheckStatus, *converge.Statistics, error) {
	stat, err := converge.CheckState(
		kubeCl, metaConfig, terraformContext,
		converge.CheckStateOptions{
			CommanderMode: c.CommanderMode,
			StateCache:    c.StateCache,
		},
	)
	if err != nil {
		return CheckStatusDestructiveOutOfSync, stat, err
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

	return checkStatus, stat, nil
}

func resolveStatisticsStatus(status string) CheckStatus {
	switch status {
	case converge.OKStatus:
		return CheckStatusInSync
	case converge.ChangedStatus:
		// NOTE: Regular out-of-sync state, which can be fixed by the converge run
		return CheckStatusOutOfSync
	case converge.DestructiveStatus:
		// NOTE: Something will be destroyed by the converge run, such change should be approved
		return CheckStatusDestructiveOutOfSync
	case converge.AbandonedStatus:
		// NOTE: Excess node — treat as destructive out-of-sync, because this node will be destroyed during converge run
		return CheckStatusDestructiveOutOfSync
	case converge.AbsentStatus:
		// NOTE: Lost node — treat as out-of-sync for now
		return CheckStatusOutOfSync
	case converge.ErrorStatus:
		// NOTE: Unknown error, probably can be healed by the retry
		return CheckStatusDestructiveOutOfSync
	}
	panic(fmt.Sprintf("unknown check infra status: %q", status))
}

func (c *Checker) kubeClient() (*client.KubernetesClient, error) {
	if c.KubeClient != nil {
		return c.KubeClient, nil
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(c.SSHClient)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
	}
	return kubeCl, nil
}
