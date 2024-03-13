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

// TODO(dhctl-for-commander): Use same tf-runner for check & converge in commander mode only, keep things as-is without changes
func (f *TerraformContext) GetCheckBaseInfraRunner(metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout), func() RunnerInterface {
			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, "base-infrastructure", opts.StateCache).
					WithSkipChangesOnDeny(true).
					WithVariables(metaConfig.MarshalConfig()).
					WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
					WithAutoApprove(opts.AutoApprove)

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown("base-infrastructure", r.Stop)
				return r
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, "base-infrastructure").
					WithVariables(metaConfig.MarshalConfig()).
					WithAutoApprove(true)
				if opts.ClusterState != nil {
					r.WithState(opts.ClusterState)
				}

				tomb.RegisterOnShutdown("base-infrastructure", r.Stop)
				return r
			}
		})
}

func (f *TerraformContext) GetCheckNodeRunner(metaConfig *config.MetaConfig, opts NodeRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithSkipChangesOnDeny(true).
					WithName(opts.NodeName).
					WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
					WithAutoApprove(opts.AutoApprove).
					WithHook(opts.ReadinessChecker)

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return r
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, opts.NodeGroupStep).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithState(opts.NodeState).
					WithName(opts.NodeName)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return r
			}
		})
}

func (f *TerraformContext) GetCheckNodeDeleteRunner(metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.node-delete.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithName(opts.NodeName).
					WithAllowedCachedState(true).
					WithSkipChangesOnDeny(true).
					WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
					WithAutoApprove(opts.AutoApprove)

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return r
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, opts.NodeGroupStep).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithName(opts.NodeName).
					WithState(opts.NodeState).
					WithSkipChangesOnDeny(true).
					WithAutoDismissDestructiveChanges(false).
					WithAutoApprove(true)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return r
			}
		})
}

// TODO: use same runner in check+converge only in commander mode, use as-is otherwise, implement destroy and bootstrap
type BaseInfraRunnerOptions struct {
	AutoDismissDestructive           bool
	AutoApprove                      bool
	CommanderMode                    bool
	StateCache                       dstate.Cache
	ClusterState                     []byte
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *TerraformContext) GetConvergeBaseInfraRunner(metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, "base-infrastructure", opts.StateCache).
				WithSkipChangesOnDeny(true).
				WithVariables(metaConfig.MarshalConfig()).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove)

			if opts.ClusterState != nil {
				r = r.WithState(opts.ClusterState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return r
		})
}

type NodeRunnerOptions struct {
	AutoDismissDestructive bool
	AutoApprove            bool

	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
	ReadinessChecker                 InfraActionHook
}

func (f *TerraformContext) GetConvergeNodeRunner(metaConfig *config.MetaConfig, opts NodeRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithSkipChangesOnDeny(true).
				WithName(opts.NodeName).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove).
				WithHook(opts.ReadinessChecker)

			if opts.NodeState != nil {
				r = r.WithState(opts.NodeState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return r
		})
}

type NodeDeleteRunnerOptions struct {
	AutoDismissDestructive bool
	AutoApprove            bool

	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *TerraformContext) GetConvergeNodeDeleteRunner(metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node-delete.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithName(opts.NodeName).
				WithAllowedCachedState(true).
				WithSkipChangesOnDeny(true).
				WithAutoDismissDestructiveChanges(opts.AutoDismissDestructive).
				WithAutoApprove(opts.AutoApprove)

			if opts.NodeState != nil {
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
