package _import

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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
		return nil, err
	}

	res := &ImportResult{}

	metaConfig, err := config.ParseConfigInCluster(kubeCl)
	if err != nil {
		return res, err
	}

	metaConfig.UUID, err = state_terraform.GetClusterUUID(kubeCl)
	if err != nil {
		return res, err
	}

	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: nil, ResetInitialState: true}); err != nil {
		return res, err
	}
	stateCache := cache.Global()

	if err = stateCache.Save("uuid", []byte(metaConfig.UUID)); err != nil {
		return res, err
	}

	if err := stateCache.SaveStruct("cluster-config", metaConfig); err != nil {
		return nil, err
	}

	if err := i.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return res, err
	}
	defer i.PhasedExecutionContext.Finalize(stateCache)

	if shouldStop, err := i.PhasedExecutionContext.StartPhase(ScanPhase, false, stateCache); err != nil {
		return res, err
	} else if shouldStop {
		return res, nil
	}

	nodesState, err := state_terraform.GetNodesStateFromCluster(kubeCl)
	if err != nil {
		return res, err
	}

	if err := stateCache.SaveStruct("nodes-state", nodesState); err != nil {
		return res, err
	}

	clusterState, err := state_terraform.GetClusterStateFromCluster(kubeCl)
	if err != nil {
		return res, err
	}

	if err := stateCache.Save("cluster-state", clusterState); err != nil {
		return nil, err
	}

	clusterConfiguration, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return res, err
	}

	providerConfiguration, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return res, err
	}

	res.Status = ImportStatusScanned
	res.ScanResult = &ScanResult{
		ClusterConfiguration:                 string(clusterConfiguration),
		ProviderSpecificClusterConfiguration: string(providerConfiguration),
	}

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CapturePhase,
		false,
		stateCache,
		PhaseData{
			ScanResult: res.ScanResult,
		},
	); err != nil {
		return res, err
	} else if shouldStop {
		return res, nil
	}

	// todo: capture
	res.Status = ImportStatusImported

	if shouldStop, err := i.PhasedExecutionContext.SwitchPhase(
		CheckPhase,
		false,
		stateCache,
		PhaseData{
			ScanResult: res.ScanResult,
		},
	); err != nil {
		return res, err
	} else if shouldStop {
		return res, nil
	}

	checker := check.NewChecker(&check.Params{
		SSHClient:     i.Params.SSHClient,
		StateCache:    stateCache,
		CommanderMode: i.Params.CommanderMode,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(res.ScanResult.ClusterConfiguration),
			[]byte(res.ScanResult.ProviderSpecificClusterConfiguration),
		),
		TerraformContext: i.Params.TerraformContext,
	})

	checkRes, err := checker.Check(ctx)
	if err != nil {
		return res, fmt.Errorf("check failed: %w", err)
	}

	if i.Params.OnCheckResult != nil {
		if err := i.Params.OnCheckResult(checkRes); err != nil {
			return res, err
		}
	}

	if err = i.PhasedExecutionContext.CompletePhase(stateCache, PhaseData{
		ScanResult:  res.ScanResult,
		CheckResult: res.CheckResult,
	}); err != nil {
		return res, err
	}

	res.CheckResult = checkRes

	return res, nil
}
