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

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/google/uuid"
	"github.com/name212/govalue"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// providerMigrationMarkerConfigMap is the ConfigMap the cloud provider module keeps in its namespace
// while a migration of the provider configuration to resources is in progress (it is
// MigrationConfigMapName of go_lib/cloud-provider/api).
const providerMigrationMarkerConfigMap = "d8-module-is-migrating"

type Params struct {
	CommanderMode         bool
	CommanderUUID         uuid.UUID
	SSHProvider           libcon.SSHProvider
	KubeProvider          libcon.KubeProvider
	OnCheckResult         func(context.Context, *check.CheckResult) error
	InfrastructureContext *infrastructure.Context
	OnPhaseFunc           OnPhaseFunc
	OnProgressFunc        phases.OnProgressFunc
	AttachResources       AttachResources
	ScanOnly              *bool
	TmpDir                string
	IsDebug               bool

	// Options carries the per-operation parsed configuration. RPC handlers
	// must populate this with a fresh *options.Options to avoid sharing global
	// state between concurrent requests.
	Options *options.Options
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
		GlobalOptions:    &i.Params.Options.Global,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		IsDebug:          i.Params.IsDebug,
	})

	i.Params.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter).
		WithUseTfCache(i.Params.Options.Cache.UseTfCache).
		WithDebug(i.Params.IsDebug)

	provider, err := i.Params.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = provider.Cleanup()
		if err != nil {
			dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Cannot cleanup provider: %v", err))
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
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Can't check attached cluster: %s", err))
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

	return kubeClient, metaConfig, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare cluster attach", func(ctx context.Context) error {
		var err error

		kubeCl, err := i.Params.KubeProvider.Client(ctx)
		if err != nil {
			return fmt.Errorf("connect to kubernetes api: %w", err)
		}
		kubeClient = &client.KubernetesClient{KubeClient: kubeCl}

		metaConfig, err = config.ParseConfigInCluster(
			ctx,
			kubeClient,
			infrastructureprovider.MetaConfigValidatorProvider(),
			nil,
			infrastructureprovider.DhctlOperationConverge,
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
		if err := cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true, Cache: i.Params.Options.Cache}); err != nil {
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

	return res, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Scan cluster", func(ctx context.Context) error {
		var err error
		stateCache := cache.Global()

		if _, err := commander.CheckShouldUpdateCommanderUUID(ctx, kubeClient, i.Params.CommanderUUID); err != nil {
			return fmt.Errorf("uuid consistency check failed: %w", err)
		}

		res = &ScanResult{}

		if err := dropLeftoverLegacyProviderConfig(ctx, kubeClient, metaConfig); err != nil {
			return fmt.Errorf("unable to check the provider configuration of the cluster: %w", err)
		}

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

		// SSHProvider is nil when helper.SSHProviderOrNil tolerated a kubeconfig-driven request
		// that supplied no master hosts.
		if govalue.NotNil(i.Params.SSHProvider) {
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
		}

		if metaConfig.ClusterType == config.StaticClusterType {
			return nil
		}

		res.SSHPublicKey, err = sshPublicKeyFromMetaConfig(metaConfig)
		if err != nil {
			return fmt.Errorf("unable to get ssh public key: %w", err)
		}

		res.Resources, err = providerResourcesYAML(metaConfig)
		if err != nil {
			return fmt.Errorf("unable to prepare provider resources: %w", err)
		}

		nodesState, err := infrastructurestate.GetNodesStateFromCluster(ctx, kubeClient)
		if err != nil {
			return fmt.Errorf("unable to get nodes tf state: %w", err)
		}

		clusterState, err := infrastructurestate.GetClusterStateFromCluster(ctx, kubeClient)
		if err != nil {
			return fmt.Errorf("unable to get cluster tf state: %w", err)
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
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Capture cluster", func(ctx context.Context) error {
		attachResources, err := template.ParseResourcesContent(
			ctx,
			i.Params.AttachResources.Template,
			i.Params.AttachResources.Values,
		)
		if err != nil {
			return fmt.Errorf("unable to parse resources: %w", err)
		}

		checkers, err := resources.GetCheckers(ctx, kubeClient, attachResources, nil)
		if err != nil {
			return fmt.Errorf("unable to get resource checkers: %w", err)
		}

		err = resources.CreateResourcesLoop(ctx, kubeClient, attachResources, checkers, nil, i.Params.Options.Bootstrap.ResourcesTimeout)
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

	return res, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Check cluster", func(ctx context.Context) error {
		var err error

		checker := check.NewChecker(&check.Params{
			KubeProvider:          kubeProvider,
			StateCache:            cache.Global(),
			CommanderMode:         i.Params.CommanderMode,
			CommanderModeParams:   checkCommanderModeParams(scanResult),
			InfrastructureContext: i.Params.InfrastructureContext,
			TmpDir:                i.Params.TmpDir,
			IsDebug:               i.Params.IsDebug,
			Embedded:              true,
			Options:               i.Params.Options,
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

// checkCommanderModeParams builds the input of the check that runs at the end of the attach. A
// cluster in the new provider format has no provider configuration document, only resources, and
// the check validates the provider from the resources it is given. The attach has no desired
// resources yet (the cluster has no template), so it gives the check what the scan read from the
// cluster: the check then compares the cluster with itself instead of failing on the missing
// ModuleConfig, credentials and NodeGroup.
func checkCommanderModeParams(scanResult *ScanResult) *commander.CommanderModeParams {
	params := commander.NewCommanderModeParams(
		[]byte(scanResult.ClusterConfiguration),
		[]byte(scanResult.ProviderSpecificClusterConfiguration),
	)

	if scanResult.Resources != "" {
		params = params.WithResources([]byte(scanResult.Resources))
	}

	return params
}

// sshPublicKeyFromMetaConfig returns the cluster ssh public key. A cluster with a legacy provider
// configuration keeps it in the provider cluster configuration. A cluster in the new provider format
// has no provider cluster configuration: the key lives in the cloud-provider module settings, and an
// absent key is not an error there (Commander treats the key as optional).
func sshPublicKeyFromMetaConfig(metaConfig *config.MetaConfig) (string, error) {
	if metaConfig.HasLegacyProviderConfig() {
		return extractSSHPublicKey(metaConfig.ProviderName, metaConfig.ProviderClusterConfig)
	}

	return sshPublicKeyFromModuleSettings(metaConfig.ProviderName, metaConfig.CloudProviderVars), nil
}

func sshPublicKeyFieldName(providerName string) string {
	if providerName == "GCP" {
		return "sshKey"
	}

	return "sshPublicKey"
}

func extractSSHPublicKey(providerName string, providerConfig map[string]json.RawMessage) (string, error) {
	key := sshPublicKeyFieldName(providerName)

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

// sshPublicKeyFromModuleSettings reads the key from the cloud-provider ModuleConfig that
// config.MetaConfig stores whole in CloudProviderVars.Settings (spec.settings.nodes.parameters.<key>).
// It returns an empty string when the settings or the key are absent.
func sshPublicKeyFromModuleSettings(providerName string, vars *config.CloudProviderVars) string {
	if vars == nil {
		return ""
	}

	// A missing or non-string key reads as absent: the key is optional.
	key, _, _ := unstructured.NestedString(vars.Settings, "spec", "settings", "nodes", "parameters", sshPublicKeyFieldName(providerName))

	return key
}

// dropLeftoverLegacyProviderConfig makes the metaConfig of a cluster migrated to the new provider
// format say so. The migration leaves the legacy provider configuration Secret in the cluster, and
// the parse of the cluster reads it next to the provider ModuleConfig; the scan would then return
// that stale document as the provider configuration of the cluster and no resources, and Commander
// would attach a cluster that runs on resources as a legacy one.
//
// A cluster counts as migrated when its provider ModuleConfig carries the node settings of the new
// model and the module no longer reports a migration in progress. A ModuleConfig that only switches
// the module on (a legacy cluster has one) carries no settings, and while the migration is in
// progress the legacy document is still the source of the configuration: both keep the legacy
// document.
func dropLeftoverLegacyProviderConfig(ctx context.Context, kubeClient *client.KubernetesClient, metaConfig *config.MetaConfig) error {
	if metaConfig.ClusterType != config.CloudClusterType || !metaConfig.HasLegacyProviderConfig() || !metaConfig.HasProviderNodeSettings() {
		return nil
	}

	pending, err := providerMigrationPending(ctx, kubeClient, metaConfig.ProviderName)
	if err != nil {
		return err
	}
	if pending {
		return nil
	}

	metaConfig.ProviderClusterConfig = nil

	return nil
}

func providerMigrationPending(ctx context.Context, kubeClient *client.KubernetesClient, providerName string) (bool, error) {
	_, err := kubeClient.CoreV1().ConfigMaps(config.CloudProviderNamespace(providerName)).Get(ctx, providerMigrationMarkerConfigMap, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get ConfigMap %q of the provider migration: %w", providerMigrationMarkerConfigMap, err)
	}

	return true, nil
}

type nodeState struct {
	Outputs struct {
		MasterIPAddressForSSH struct {
			Value string `json:"value,omitempty"`
		} `json:"master_ip_address_for_ssh"`
	} `json:"outputs"`
}
