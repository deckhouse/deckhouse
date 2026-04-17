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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type Params struct {
	CommanderMode         bool
	CommanderUUID         uuid.UUID
	SSHProvider           libcon.SSHProvider
	KubeProvider          libcon.KubeProvider
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

	if err = i.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return nil, err
	}
	defer func() {
		_ = i.PhasedExecutionContext.Finalize(ctx, stateCache)
	}()

	if shouldStop, err := i.PhasedExecutionContext.StartPhase(ctx, phases.CommanderAttachScanPhase, false, stateCache); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &AttachResult{}, nil
	}

	scanResult, err := i.scan(ctx, kubeClient, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to scan cluster: %w", err)
	}

	if ptr.Deref(i.Params.ScanOnly, true) {
		if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(ctx, stateCache, PhaseData{
			ScanResult: scanResult,
		}); err != nil {
			return nil, fmt.Errorf("unable to complete phase: %w", err)
		}
		return &AttachResult{Status: StatusScanned, ScanResult: scanResult}, nil
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		ctx,
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
		ctx,
		phases.CommanderAttachCheckPhase,
		false,
		stateCache,
		PhaseData{},
	); err != nil {
		return nil, fmt.Errorf("unable to switch phase: %w", err)
	} else if shouldStop {
		return &AttachResult{Status: StatusAttached}, nil
	}

	checkResult, err := i.check(ctx, i.Params.KubeProvider, scanResult)
	if err != nil {
		// check is optional
		log.WarnF("Can't check attached cluster: %s\n", err)
	}

	if err = i.PhasedExecutionContext.CompletePhaseAndPipeline(ctx, stateCache, PhaseData{
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

	return kubeClient, metaConfig, log.ProcessCtx(ctx, "attach", "Prepare cluster attach", func(ctx context.Context) error {
		var err error

		kubeCl, err := i.Params.KubeProvider.Client(ctx)
		if err != nil {
			return fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
		}

		kubeClient = &client.KubernetesClient{KubeClient: kubeCl}

		metaConfig, err = config.ParseConfigInCluster(
			ctx,
			kubeClient,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(i.Params.Logger),
			),
			nil,
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
		if err := cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true}); err != nil {
			return fmt.Errorf("unable to init cache: %w", err)
		}

		return nil
	})
}

func (i *Attacher) scan(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) (*ScanResult, error) {
	var res *ScanResult

	return res, log.ProcessCtx(ctx, "commander/attach", "Scan cluster", func(ctx context.Context) error {
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

		if err = stateCache.Save(ctx, "uuid", []byte(metaConfig.UUID)); err != nil {
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

		sshCl, err := i.Params.SSHProvider.Client(ctx)
		if err != nil {
			return err
		}

		// TODO keep keys in session instead of ReadFile
		if len(sshCl.PrivateKeys()) > 0 {
			sshPrivateKey, err := os.ReadFile(sshCl.PrivateKeys()[0].Key)
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

		if err = stateCache.Save(ctx, "base-infrastructure.tfstate", clusterState); err != nil {
			return fmt.Errorf("unable to save cluster tf state to cache: %w", err)
		}

		hosts := map[string]string{}
		for _, ngState := range nodesState {
			for node, nState := range ngState.State {
				key := fmt.Sprintf("%s.tfstate", node)
				if err = stateCache.Save(ctx, key, nState); err != nil {
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

		err = stateCache.SaveStruct(ctx, "cluster-hosts", hosts)
		if err != nil {
			return fmt.Errorf("unable to save master ssh hosts: %w", err)
		}

		return nil
	})
}

func (i *Attacher) capture(
	ctx context.Context,
	kubeClient *client.KubernetesClient,
) error {
	return log.ProcessCtx(ctx, "commander/attach", "Capture cluster", func(ctx context.Context) error {
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
	kubeProvider libcon.KubeProvider,
	scanResult *ScanResult,
) (*check.CheckResult, error) {
	var res *check.CheckResult

	return res, log.ProcessCtx(ctx, "commander/attach", "Check cluster", func(ctx context.Context) error {
		var err error

		checker := check.NewChecker(&check.Params{
			KubeProvider:  kubeProvider,
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
			Embedded:              true,
		})

		checker.SetExternalPhasedContext(i.PhasedExecutionContext)

		// provider will cleanup in Attach
		res, _, err = checker.Check(ctx)
		if err != nil {
			return fmt.Errorf("unable to check cluster state: %w", err)
		}

		if i.Params.OnCheckResult != nil {
			if err = i.Params.OnCheckResult(ctx, res); err != nil {
				return fmt.Errorf("OnCheckResult error: %w", err)
			}
		}

		return nil
	})
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
