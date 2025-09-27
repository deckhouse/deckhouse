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

package attach

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type Params struct {
	CommanderMode         bool
	CommanderUUID         uuid.UUID
	SSHClient             node.SSHClient
	OnCheckResult         func(*check.CheckResult) error
	InfrastructureContext *infrastructure.Context
	OnPhaseFunc           OnPhaseFunc
	OnProgressFunc        phases.OnProgressFunc
	AttachResources       AttachResources
	ScanOnly              *bool
	TmpDir                string
	Logger                log.Logger
	IsDebug               bool
}

type AttachResources struct {
	Template string
	Values   map[string]any
}

type Attacher struct {
	Params                 *Params
	PhasedExecutionContext phases.PhasedExecutionContext[PhaseData]
}

func NewAttacher(params *Params) *Attacher {
	if !params.CommanderMode {
		panic("attach commander operation supported only in commander mode")
	}

	// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
	// if params.CommanderUUID == uuid.Nil {
	//	panic("CommanderUUID required for commander/attach operation!")
	// }

	return &Attacher{
		Params: params,
		PhasedExecutionContext: phases.NewPhasedExecutionContext[PhaseData](
			phases.OperationCommanderAttach, params.OnPhaseFunc, params.OnProgressFunc,
		),
	}
}

func (i *Attacher) Attach(ctx context.Context) (*AttachResult, error) {
	kubeClient, metaConfig, err := i.prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare cluster attach to commander: %w", err)
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           i.Params.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           i.Params.Logger,
		IsDebug:          i.Params.IsDebug,
	})

	i.Params.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, i.Params.Logger)

	provider, err := i.Params.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = provider.Cleanup()
		if err != nil {
			i.Params.Logger.LogErrorF("Cannot cleanup provider: %v\n", err)
			return
		}
	}()

	stateCache := cache.Global()

	if err = i.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	defer i.PhasedExecutionContext.Finalize(stateCache)

	if shouldStop, err := i.PhasedExecutionContext.StartPhase(phases.CommanderAttachScanPhase, false, stateCache); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &AttachResult{}, nil
	}

	scanResult, err := i.scan(ctx, kubeClient, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to scan cluster: %w", err)
	}

	if ptr.Deref(i.Params.ScanOnly, true) {
		if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, PhaseData{
			ScanResult: scanResult,
		}); err != nil {
			return nil, fmt.Errorf("unable to complete phase: %w", err)
		}
		return &AttachResult{Status: StatusScanned, ScanResult: scanResult}, nil
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		phases.CommanderAttachCapturePhase,
		false,
		stateCache,
		PhaseData{ScanResult: scanResult},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &AttachResult{Status: StatusScanned, ScanResult: scanResult}, nil
	}

	err = i.capture(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("unable to capture cluster: %w", err)
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		phases.CommanderAttachCheckPhase,
		false,
		stateCache,
		PhaseData{},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &AttachResult{Status: StatusAttached}, nil
	}

	checkResult, err := i.check(ctx, kubeClient, scanResult)
	if err != nil {
		// check is optional
		log.WarnF("Can't check attached cluster: %s\n", err)
	}

	if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, PhaseData{
		CheckResult: checkResult,
	}); err != nil {
		return nil, fmt.Errorf("unable to complete phase: %w", err)
	}

	return &AttachResult{
		Status:      StatusAttached,
		ScanResult:  scanResult,
		CheckResult: checkResult,
	}, nil
}

func (i *Attacher) prepare(ctx context.Context) (*client.KubernetesClient, *config.MetaConfig, error) {
	var (
		kubeClient *client.KubernetesClient
		metaConfig *config.MetaConfig
	)

	err := log.Process("attach", "Prepare cluster attach", func() error {
		var err error

		kubeClient, err = kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(i.Params.SSHClient))
		if err != nil {
			return fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
		}

		metaConfig, err = config.ParseConfigInCluster(
			ctx,
			kubeClient,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(i.Params.Logger),
			),
		)
		if err != nil {
			return fmt.Errorf("unable to parse cluster config: %w", err)
		}

		if _, err := metaConfig.GetFullUUID(); err != nil || metaConfig.UUID == "" {
			u, err := infrastructurestate.GetClusterUUID(ctx, kubeClient)
			if err != nil {
				return err
			}

			metaConfig.UUID = u
		}

		cachePath := metaConfig.CachePath()
		if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true}); err != nil {
			return fmt.Errorf("unable to init cache: %w", err)
		}

		return nil
	})

	return kubeClient, metaConfig, err
}

func (i *Attacher) scan(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) (*ScanResult, error) {
	var res *ScanResult

	err := log.Process("commander/attach", "Scan cluster", func() error {
		var err error
		stateCache := cache.Global()

		if _, err := commander.CheckShouldUpdateCommanderUUID(ctx, kubeClient, i.Params.CommanderUUID); err != nil {
			return fmt.Errorf("uuid consistency check failed: %w", err)
		}

		res = &ScanResult{}

		metaConfig.UUID, err = infrastructurestate.GetClusterUUID(ctx, kubeClient)
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
		res.ClusterConfiguration = string(clusterConfiguration)

		providerConfiguration, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("unable to prepare provider cluster config yaml: %w", err)
		}
		res.ProviderSpecificClusterConfiguration = string(providerConfiguration)

		if len(i.Params.SSHClient.PrivateKeys()) > 0 {
			sshPrivateKey, err := os.ReadFile(i.Params.SSHClient.PrivateKeys()[0].Key)
			if err != nil {
				return fmt.Errorf("unable to read ssh private key: %w", err)
			}
			res.SSHPrivateKey = string(sshPrivateKey)
		}

		if metaConfig.ClusterType == config.StaticClusterType {
			return nil
		}

		res.SSHPublicKey, err = extractSSHPublicKey(metaConfig.ProviderName, metaConfig.ProviderClusterConfig)
		if err != nil {
			return fmt.Errorf("unable to get ssh public key: %w", err)
		}

		nodesState, err := infrastructurestate.GetNodesStateFromCluster(ctx, kubeClient)
		if err != nil {
			return fmt.Errorf("unable to get nodes tf state: %w", err)
		}

		clusterState, err := infrastructurestate.GetClusterStateFromCluster(ctx, kubeClient)
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

func (i *Attacher) capture(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
) error {
	return log.Process("commander/attach", "Capture cluster", func() error {
		attachResources, err := template.ParseResourcesContent(
			i.Params.AttachResources.Template,
			i.Params.AttachResources.Values,
		)
		if err != nil {
			return fmt.Errorf("unable to parse resources: %w", err)
		}

		checkers, err := resources.GetCheckers(kubeClient, attachResources, nil)
		if err != nil {
			return fmt.Errorf("unable to get resource checkers: %w", err)
		}

		err = resources.CreateResourcesLoop(ctx, kubeClient, attachResources, checkers, nil)
		if err != nil {
			return fmt.Errorf("unable to create resources: %w", err)
		}

		return nil
	})
}

func (i *Attacher) check(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
	scanResult *ScanResult,
) (*check.CheckResult, error) {
	var res *check.CheckResult

	err := log.Process("commander/attach", "Check cluster", func() error {
		var err error

		checker := check.NewChecker(&check.Params{
			KubeClient:    kubeClient,
			StateCache:    cache.Global(),
			CommanderMode: i.Params.CommanderMode,
			CommanderModeParams: commander.NewCommanderModeParams(
				[]byte(scanResult.ClusterConfiguration),
				[]byte(scanResult.ProviderSpecificClusterConfiguration),
			),
			InfrastructureContext: i.Params.InfrastructureContext,
			TmpDir:                i.Params.TmpDir,
			IsDebug:               i.Params.IsDebug,
			Logger:                i.Params.Logger,
		})

		// provider will cleanup in Attach
		res, _, err = checker.Check(ctx)
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

func extractSSHPublicKey(providerName string, providerConfig map[string]json.RawMessage) (string, error) {
	var key string
	switch providerName {
	case "GCP":
		key = "sshKey"
	default:
		key = "sshPublicKey"
	}

	sshKeyJSON, ok := providerConfig[key]
	if !ok {
		return "", fmt.Errorf("%s not found in cloud provider config", key)
	}

	var sshKey string
	err := json.Unmarshal(sshKeyJSON, &sshKey)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal %s: %w", key, err)
	}

	return sshKey, nil
}

type nodeState struct {
	Outputs struct {
		MasterIPAddressForSSH struct {
			Value string `json:"value,omitempty"`
		} `json:"master_ip_address_for_ssh,omitempty"`
	} `json:"outputs,omitempty"`
}
