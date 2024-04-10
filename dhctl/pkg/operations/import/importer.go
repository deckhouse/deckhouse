// Copyright 2024 Flant JSC
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

package _import

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type Params struct {
	CommanderMode    bool
	SSHClient        *ssh.Client
	OnCheckResult    func(*check.CheckResult) error
	TerraformContext *terraform.TerraformContext
	OnPhaseFunc      OnPhaseFunc
}

type Importer struct {
	Params                 *Params
	PhasedExecutionContext phases.PhasedExecutionContext[PhaseData]
}

func NewImporter(params *Params) *Importer {
	if !params.CommanderMode {
		panic("import operation currently supported only in commander mode")
	}

	return &Importer{
		Params:                 params,
		PhasedExecutionContext: phases.NewPhasedExecutionContext[PhaseData](params.OnPhaseFunc),
	}
}

func (i *Importer) Import(ctx context.Context) (*ImportResult, error) {
	kubeCl, err := operations.ConnectToKubernetesAPI(i.Params.SSHClient)
	if err != nil {
		return nil, fmt.Errorf("unable to create k8s client: %w", err)
	}

	metaConfig, err := config.ParseConfigInCluster(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cluster config: %w", err)
	}

	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true}); err != nil {
		return nil, err
	}
	stateCache := cache.Global()

	if err = i.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	defer i.PhasedExecutionContext.Finalize(stateCache)

	if shouldStop, err := i.PhasedExecutionContext.StartPhase(ScanPhase, false, stateCache); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return nil, nil
	}

	res := &ImportResult{}

	res.ScanResult, err = i.scan(ctx, kubeCl, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to scan cluster: %w", err)
	}
	res.Status = ImportStatusScanned

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CapturePhase,
		false,
		stateCache,
		PhaseData{
			ScanResult: res.ScanResult,
		},
	); err != nil {
		return nil, err
	} else if shouldStop {
		return res, nil
	}

	err = i.capture(ctx, kubeCl, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to capture cluster: %w", err)
	}
	res.Status = ImportStatusImported

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CheckPhase,
		false,
		stateCache,
		PhaseData{
			ScanResult: res.ScanResult,
		},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return res, nil
	}

	res.CheckResult, err = i.check(ctx, res.ScanResult)
	if err != nil {
		// check is optional
		log.WarnF("Can't check imported cluster: %s\n", err)
	}

	if err = i.PhasedExecutionContext.CompletePhase(stateCache, PhaseData{
		ScanResult:  res.ScanResult,
		CheckResult: res.CheckResult,
	}); err != nil {
		return nil, fmt.Errorf("unable to complete phase: %w", err)
	}

	return res, nil
}

func (i *Importer) scan(
	_ context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) (*ScanResult, error) {
	var err error
	stateCache := cache.Global()

	metaConfig.UUID, err = state_terraform.GetClusterUUID(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster uuid: %w", err)
	}

	if err = stateCache.Save("uuid", []byte(metaConfig.UUID)); err != nil {
		return nil, fmt.Errorf("unable to save cluster uuid to cache: %w", err)
	}

	if err = stateCache.SaveStruct("cluster-config", metaConfig); err != nil {
		return nil, fmt.Errorf("unable to save cluster config to cache: %w", err)
	}

	nodesState, err := state_terraform.GetNodesStateFromCluster(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("unable to get nodes tf state: %w", err)
	}

	if err = stateCache.SaveStruct("nodes-state", nodesState); err != nil {
		return nil, fmt.Errorf("unable to save nodes tf state to cache: %w", err)
	}

	clusterState, err := state_terraform.GetClusterStateFromCluster(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("unable get cluster tf state: %w", err)
	}

	if err = stateCache.Save("cluster-state", clusterState); err != nil {
		return nil, fmt.Errorf("unable to save cluster tf state to cache: %w", err)
	}

	clusterConfiguration, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("unable to prepare cluster config yaml: %w", err)
	}

	providerConfiguration, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("unable to prepare provider cluster config yaml: %w", err)
	}

	return &ScanResult{
		ClusterConfiguration:                 string(clusterConfiguration),
		ProviderSpecificClusterConfiguration: string(providerConfiguration),
	}, nil
}

func (i *Importer) capture(
	_ context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) error {
	// todo: capture
	var err error
	stateCache := cache.Global()

	_ = err
	_ = stateCache

	return nil
}

func (i *Importer) check(
	ctx context.Context,
	scanResult *ScanResult,
) (*check.CheckResult, error) {
	var err error

	checker := check.NewChecker(&check.Params{
		SSHClient:     i.Params.SSHClient,
		StateCache:    cache.Global(),
		CommanderMode: i.Params.CommanderMode,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(scanResult.ClusterConfiguration),
			[]byte(scanResult.ProviderSpecificClusterConfiguration),
		),
		TerraformContext: i.Params.TerraformContext,
	})

	res, err := checker.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to check cluster state: %w", err)
	}

	if i.Params.OnCheckResult != nil {
		if err = i.Params.OnCheckResult(res); err != nil {
			return nil, fmt.Errorf("OnCheckResult error: %w", err)
		}
	}

	return res, nil
}
