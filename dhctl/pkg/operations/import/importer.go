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
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"k8s.io/utils/pointer"
)

type Params struct {
	CommanderMode    bool
	SSHClient        *ssh.Client
	OnCheckResult    func(*check.CheckResult) error
	TerraformContext *terraform.TerraformContext
	OnPhaseFunc      OnPhaseFunc
	ImportResources  ImportResources
	ScanOnly         *bool
}

type ImportResources struct {
	Template string
	Values   map[string]any
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
	kubeClient, metaConfig, err := i.prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare cluster import: %w", err)
	}

	stateCache := cache.Global()

	if err = i.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	defer i.PhasedExecutionContext.Finalize(stateCache)

	if shouldStop, err := i.PhasedExecutionContext.StartPhase(ScanPhase, false, stateCache); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &ImportResult{}, nil
	}

	scanResult, err := i.scan(ctx, kubeClient, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to scan cluster: %w", err)
	}

	if pointer.BoolDeref(i.Params.ScanOnly, true) {
		if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, PhaseData{
			ScanResult: scanResult,
		}); err != nil {
			return nil, fmt.Errorf("unable to complete phase: %w", err)
		}
		return &ImportResult{Status: ImportStatusScanned, ScanResult: scanResult}, nil
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CapturePhase,
		false,
		stateCache,
		PhaseData{ScanResult: scanResult},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &ImportResult{Status: ImportStatusScanned, ScanResult: scanResult}, nil
	}

	err = i.capture(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("unable to capture cluster: %w", err)
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CheckPhase,
		false,
		stateCache,
		PhaseData{},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &ImportResult{Status: ImportStatusImported}, nil
	}

	checkResult, err := i.check(ctx, kubeClient, scanResult)
	if err != nil {
		// check is optional
		log.WarnF("Can't check imported cluster: %s\n", err)
	}

	if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, PhaseData{
		CheckResult: checkResult,
	}); err != nil {
		return nil, fmt.Errorf("unable to complete phase: %w", err)
	}

	return &ImportResult{
		Status:      ImportStatusImported,
		ScanResult:  scanResult,
		CheckResult: checkResult,
	}, nil
}

func (i *Importer) prepare(_ context.Context) (*client.KubernetesClient, *config.MetaConfig, error) {
	var (
		kubeClient *client.KubernetesClient
		metaConfig *config.MetaConfig
	)

	err := log.Process("import", "Prepare cluster import", func() error {
		var err error

		kubeClient, err = operations.ConnectToKubernetesAPI(i.Params.SSHClient)
		if err != nil {
			return fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
		}

		metaConfig, err = config.ParseConfigInCluster(kubeClient)
		if err != nil {
			return fmt.Errorf("unable to parse cluster config: %w", err)
		}

		cachePath := metaConfig.CachePath()
		if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true}); err != nil {
			return fmt.Errorf("unable to init cache: %w", err)
		}

		return nil
	})

	return kubeClient, metaConfig, err
}

func (i *Importer) scan(
	_ context.Context,
	kubeClient *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) (*ScanResult, error) {
	var res *ScanResult

	err := log.Process("import", "Scan cluster", func() error {
		var err error
		stateCache := cache.Global()

		metaConfig.UUID, err = state_terraform.GetClusterUUID(kubeClient)
		if err != nil {
			return fmt.Errorf("unable to get cluster uuid: %w", err)
		}

		if err = stateCache.Save("uuid", []byte(metaConfig.UUID)); err != nil {
			return fmt.Errorf("unable to save cluster uuid to cache: %w", err)
		}

		clusterConfiguration, err := metaConfig.ClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("unable to prepare cluster config yaml: %w", err)
		}

		providerConfiguration, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("unable to prepare provider cluster config yaml: %w", err)
		}

		res = &ScanResult{
			ClusterConfiguration:                 string(clusterConfiguration),
			ProviderSpecificClusterConfiguration: string(providerConfiguration),
		}

		if metaConfig.ClusterType == config.StaticClusterType {
			return nil
		}

		nodesState, err := state_terraform.GetNodesStateFromCluster(kubeClient)
		if err != nil {
			return fmt.Errorf("unable to get nodes tf state: %w", err)
		}

		clusterState, err := state_terraform.GetClusterStateFromCluster(kubeClient)
		if err != nil {
			return fmt.Errorf("unable get cluster tf state: %w", err)
		}

		if err = stateCache.Save("base-infrastructure.tfstate", clusterState); err != nil {
			return fmt.Errorf("unable to save cluster tf state to cache: %w", err)
		}

		hosts := map[string]string{}
		for _, ngState := range nodesState {
			for node, nState := range ngState.State {
				key := fmt.Sprintf("%s.tfstate", node)
				if err = stateCache.Save(key, nState); err != nil {
					return fmt.Errorf("unable to save node tf state to cache: %w", err)
				}

				state := nodeState{}
				err = json.Unmarshal(nState, &state)
				if err != nil {
					return fmt.Errorf("unable to parse master ssh hosts: %w", err)
				}
				if state.Outputs.MasterIPAddressForSSH.Value != "" {
					hosts[node] = state.Outputs.MasterIPAddressForSSH.Value
				}
			}
		}

		err = stateCache.SaveStruct("cluster-hosts", hosts)
		if err != nil {
			return fmt.Errorf("unable to save master ssh hosts: %w", err)
		}

		return nil
	})

	return res, err
}

func (i *Importer) capture(
	_ context.Context,
	kubeClient *client.KubernetesClient,
) error {
	return log.Process("import", "Capture cluster", func() error {
		importResources, err := template.ParseResourcesContent(
			i.Params.ImportResources.Template,
			i.Params.ImportResources.Values,
		)
		if err != nil {
			return fmt.Errorf("unable to parse resources: %w", err)
		}

		checkers, err := resources.GetCheckers(kubeClient, importResources, nil)
		if err != nil {
			return fmt.Errorf("unable to get resource checkers: %w", err)
		}

		err = resources.CreateResourcesLoop(kubeClient, importResources, checkers)
		if err != nil {
			return fmt.Errorf("unable to create resources: %w", err)
		}

		return nil
	})
}

func (i *Importer) check(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
	scanResult *ScanResult,
) (*check.CheckResult, error) {
	var res *check.CheckResult

	err := log.Process("import", "Check cluster", func() error {
		var err error

		checker := check.NewChecker(&check.Params{
			KubeClient:    kubeClient,
			StateCache:    cache.Global(),
			CommanderMode: i.Params.CommanderMode,
			CommanderModeParams: commander.NewCommanderModeParams(
				[]byte(scanResult.ClusterConfiguration),
				[]byte(scanResult.ProviderSpecificClusterConfiguration),
			),
			TerraformContext: i.Params.TerraformContext,
		})

		res, err = checker.Check(ctx)
		if err != nil {
			return fmt.Errorf("unable to check cluster state: %w", err)
		}

		if i.Params.OnCheckResult != nil {
			if err = i.Params.OnCheckResult(res); err != nil {
				return fmt.Errorf("OnCheckResult error: %w", err)
			}
		}

		return nil
	})

	return res, err
}

type nodeState struct {
	Outputs struct {
		MasterIPAddressForSSH struct {
			Value string `json:"value,omitempty"`
		} `json:"master_ip_address_for_ssh,omitempty"`
	} `json:"outputs,omitempty"`
}
