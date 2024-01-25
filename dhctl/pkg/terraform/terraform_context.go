package terraform

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type TerraformContext struct {
	terraformRunnerByName    map[string]RunnerInterface
	terraformRunnerByNameMux sync.Mutex
}

func NewTerraformContext() *TerraformContext {
	return &TerraformContext{
		terraformRunnerByName: make(map[string]RunnerInterface),
	}
}

func (f *TerraformContext) getOrCreateRunner(name string, createFunc func() RunnerInterface) RunnerInterface {
	f.terraformRunnerByNameMux.Lock()
	defer f.terraformRunnerByNameMux.Unlock()

	// FIXME
	//if r, hasKey := f.terraformRunnerByName[name]; hasKey {
	//	return r
	//}

	r := createFunc()
	f.terraformRunnerByName[name] = r
	return r
}

type CheckBaseInfraRunnerOptions struct {
	CommanderMode bool
	ClusterState  []byte
}

// TODO(dhctl-for-commander): Use same tf-runner for check & converge in commander mode only, keep things as-is without changes
func (f *TerraformContext) GetCheckBaseInfraRunner(metaConfig *config.MetaConfig, opts CheckBaseInfraRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout), func() RunnerInterface {
			r := NewImmutableRunnerFromConfig(metaConfig, "base-infrastructure").
				WithVariables(metaConfig.MarshalConfig()).
				WithState(opts.ClusterState).
				WithAutoApprove(true)

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return r
		})
}

type CheckNodeRunnerOptions struct {
	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string
}

func (f *TerraformContext) GetCheckNodeRunner(metaConfig *config.MetaConfig, opts CheckNodeRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			r := NewImmutableRunnerFromConfig(metaConfig, opts.NodeGroupStep).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithState(opts.NodeState).
				WithName(opts.NodeName)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return r
		})
}

// TODO: use same runner in check+converge only in commander mode, use as-is otherwise, implement destroy and bootstrap

type ConvergeBaseInfraRunnerOptions struct {
	AutoDismissDestructive           bool
	AutoApprove                      bool
	CommanderMode                    bool
	ClusterState                     []byte
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *TerraformContext) GetConvergeBaseInfraRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts ConvergeBaseInfraRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache).
				WithSkipChangesOnDeny(true).
				WithVariables(metaConfig.MarshalConfig()).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove)
			if !opts.CommanderMode {
				r = r.WithState(opts.ClusterState)
			}
			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return r
		})
}

type ConvergeNodeRunnerOptions struct {
	AutoDismissDestructive bool
	AutoApprove            bool

	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	AdditionalStateSaverDestinations []SaverDestination
	ReadinessChecker                 InfraActionHook
}

func (f *TerraformContext) GetConvergeNodeRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts ConvergeNodeRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithSkipChangesOnDeny(true).
				WithName(opts.NodeName).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove).
				WithHook(opts.ReadinessChecker)
			if !opts.CommanderMode {
				r = r.WithState(opts.NodeState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return r
		})
}

type ConvergeNodeDeleteRunnerOptions struct {
	AutoDismissDestructive bool
	AutoApprove            bool

	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *TerraformContext) GetConvergeNodeDeleteRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts ConvergeNodeDeleteRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node-delete.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithName(opts.NodeName).
				WithAllowedCachedState(true).
				WithSkipChangesOnDeny(true).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove)
			if !opts.CommanderMode {
				r = r.WithState(opts.NodeState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return r
		})
}

func (f *TerraformContext) GetBootstrapBaseInfraRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("bootstrap.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache).
				WithVariables(metaConfig.MarshalConfig()).
				WithAutoApprove(true)

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return r
		},
	)
}

type BootstrapNodeRunnerOptions struct {
	AutoApprove                      bool
	NodeName                         string
	NodeGroupName                    string
	NodeGroupStep                    string
	NodeIndex                        int
	NodeCloudConfig                  string
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *TerraformContext) GetBootstrapNodeRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts BootstrapNodeRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("bootstrap.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			nodeConfig := metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)

			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache).
				WithVariables(nodeConfig).
				WithName(opts.NodeName).
				WithAutoApprove(opts.AutoApprove).
				WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return r
		},
	)
}

type DestroyBaseInfraRunnerOptions struct {
	AutoApprove bool
}

func (f *TerraformContext) GetDestroyBaseInfraRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyBaseInfraRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("destroy.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			runner := NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache).
				WithVariables(metaConfig.MarshalConfig()).
				WithAllowedCachedState(true).
				WithAutoApprove(opts.AutoApprove)

			tomb.RegisterOnShutdown("base-infrastructure", runner.Stop)

			return runner
		},
	)
}

type DestroyNodeRunnerOptions struct {
	AutoApprove   bool
	NodeName      string
	NodeGroupName string
	NodeGroupStep string
	NodeIndex     int
}

func (f *TerraformContext) GetDestroyNodeRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyNodeRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("destroy.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			runner := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, "")).
				WithName(opts.NodeName).
				WithAllowedCachedState(true).
				WithAutoApprove(opts.AutoApprove)

			tomb.RegisterOnShutdown(opts.NodeName, runner.Stop)

			return runner
		},
	)
}
