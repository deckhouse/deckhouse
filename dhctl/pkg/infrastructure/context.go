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

package infrastructure

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type Context struct {
	infrastructureRunnerByName    map[string]RunnerInterface
	infrastructureRunnerByNameMux sync.Mutex
	provider                      ExecutorProvider
	stateChecker                  StateChecker
}

func NewContextWithProvider(provider ExecutorProvider) *Context {
	return &Context{
		infrastructureRunnerByName: make(map[string]RunnerInterface),
		provider:                   provider,
	}
}

func NewContext() *Context {
	return &Context{
		infrastructureRunnerByName: make(map[string]RunnerInterface),
	}
}

func (f *Context) SetExecutorProvider(provider ExecutorProvider) {
	f.provider = provider
}

func (f *Context) WithStateChecker(checker StateChecker) {
	f.stateChecker = checker
}

func (f *Context) getOrCreateRunner(name string, createFunc func() RunnerInterface) RunnerInterface {
	f.infrastructureRunnerByNameMux.Lock()
	defer f.infrastructureRunnerByNameMux.Unlock()

	// FIXME
	//if r, hasKey := f.infrastructureRunnerByNameMux[name]; hasKey {
	//	return r
	//}

	r := createFunc()
	f.infrastructureRunnerByName[name] = r
	return r
}

func applyAutomaticSettingsForChangesRunner(r *Runner, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(true).
		WithAutoApprove(false).
		WithSkipChangesOnDeny(true).
		WithAutoDismissChanges(true)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticSettings(r *Runner, settings AutomaticSettings, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(settings.AutoDismissDestructive).
		WithAutoApprove(settings.AutoApprove).
		WithAutoDismissChanges(settings.AutoDismissChanges)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticSettingsForBootstrap(r *Runner, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(false).
		WithAutoApprove(true).
		WithAutoDismissChanges(false)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticApproveSettings(r *Runner, settings AutoApproveSettings, stateChecker StateChecker) *Runner {
	r.WithAutoApprove(settings.AutoApprove)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

// TODO(dhctl-for-commander): Use same tf-runner for check & converge in commander mode only, keep things as-is without changes
func (f *Context) GetCheckBaseInfraRunner(metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout), func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, "base-infrastructure", opts.StateCache, f.provider).
					WithVariables(metaConfig.MarshalConfig())

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown("base-infrastructure", r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, "base-infrastructure", f.provider).
					WithVariables(metaConfig.MarshalConfig())
				if opts.ClusterState != nil {
					r.WithState(opts.ClusterState)
				}

				tomb.RegisterOnShutdown("base-infrastructure", r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			}
		})
}

func (f *Context) GetCheckNodeRunner(metaConfig *config.MetaConfig, opts NodeRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache, f.provider).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithName(opts.NodeName).
					WithHook(opts.Hook)

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, opts.NodeGroupStep, f.provider).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithState(opts.NodeState).
					WithName(opts.NodeName)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			}
		})
}

func (f *Context) GetCheckNodeDeleteRunner(metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("check.node-delete.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.LayoutStep),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}
			if opts.CommanderMode {
				r := NewRunnerFromConfig(metaConfig, opts.LayoutStep, opts.StateCache, f.provider).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithName(opts.NodeName).
					WithAllowedCachedState(true)

				r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			} else {
				r := NewImmutableRunnerFromConfig(metaConfig, opts.LayoutStep, f.provider).
					WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
					WithName(opts.NodeName).
					WithState(opts.NodeState)

				tomb.RegisterOnShutdown(opts.NodeName, r.Stop)
				return applyAutomaticSettingsForChangesRunner(r, f.stateChecker)
			}
		})
}

// TODO: use same runner in check+converge only in commander mode, use as-is otherwise, implement destroy and bootstrap
type BaseInfraRunnerOptions struct {
	CommanderMode                    bool
	StateCache                       dstate.Cache
	ClusterState                     []byte
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *Context) GetConvergeBaseInfraRunner(metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions, automaticSettings AutomaticSettings) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			r := NewRunnerFromConfig(metaConfig, "base-infrastructure", opts.StateCache, f.provider).
				WithSkipChangesOnDeny(true).
				WithVariables(metaConfig.MarshalConfig())
			if opts.ClusterState != nil {
				r = r.WithState(opts.ClusterState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return applyAutomaticSettings(r, automaticSettings, f.stateChecker)
		})
}

type NodeRunnerOptions struct {
	NodeName        string
	NodeGroupName   string
	NodeGroupStep   string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
	Hook                             InfraActionHook
}

func (f *Context) GetConvergeNodeRunner(metaConfig *config.MetaConfig, opts NodeRunnerOptions, automaticSettings AutomaticSettings) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, opts.StateCache, f.provider).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithSkipChangesOnDeny(true).
				WithName(opts.NodeName).
				WithHook(opts.Hook)

			if opts.NodeState != nil {
				r = r.WithState(opts.NodeState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return applyAutomaticSettings(r, automaticSettings, f.stateChecker)
		})
}

type NodeDeleteRunnerOptions struct {
	NodeName        string
	NodeGroupName   string
	LayoutStep      string
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
	Hook                             InfraActionHook
}

func (f *Context) GetConvergeNodeDeleteRunner(metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions, automaticSettings AutomaticSettings) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("converge.node-delete.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.LayoutStep),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			r := NewRunnerFromConfig(metaConfig, opts.LayoutStep, opts.StateCache, f.provider).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
				WithName(opts.NodeName).
				WithAllowedCachedState(true).
				WithSkipChangesOnDeny(true).
				WithHook(opts.Hook)

			if opts.NodeState != nil {
				r = r.WithState(opts.NodeState)
			}

			r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return applyAutomaticSettings(r, automaticSettings, f.stateChecker)
		})
}

func (f *Context) GetBootstrapBaseInfraRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache) RunnerInterface {
	return f.getOrCreateRunner(
		fmt.Sprintf("bootstrap.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout),
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			r := NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache, f.provider).
				WithVariables(metaConfig.MarshalConfig())

			tomb.RegisterOnShutdown("base-infrastructure", r.Stop)

			return applyAutomaticSettingsForBootstrap(r, f.stateChecker)
		},
	)
}

type BootstrapNodeRunnerOptions struct {
	NodeName                         string
	NodeGroupName                    string
	NodeGroupStep                    string
	NodeIndex                        int
	NodeCloudConfig                  string
	AdditionalStateSaverDestinations []SaverDestination
	RunnerLogger                     log.Logger
}

func (f *Context) GetBootstrapNodeRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts BootstrapNodeRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("bootstrap.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			nodeConfig := metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)

			r := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache, f.provider).
				WithVariables(nodeConfig).
				WithName(opts.NodeName).
				WithLogger(opts.RunnerLogger)

			tomb.RegisterOnShutdown(opts.NodeName, r.Stop)

			return applyAutomaticSettingsForBootstrap(r, f.stateChecker)
		},
	)
}

type DestroyBaseInfraRunnerOptions struct {
	AutoApproveSettings
}

func (f *Context) GetDestroyBaseInfraRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyBaseInfraRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("destroy.base-infrastructure.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			runner := NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache, f.provider).
				WithVariables(metaConfig.MarshalConfig()).
				WithAllowedCachedState(true)

			tomb.RegisterOnShutdown("base-infrastructure", runner.Stop)

			return applyAutomaticApproveSettings(runner, opts.AutoApproveSettings, f.stateChecker)
		},
	)
}

type DestroyNodeRunnerOptions struct {
	AutoApproveSettings

	NodeName      string
	NodeGroupName string
	NodeGroupStep string
	NodeIndex     int
}

func (f *Context) GetDestroyNodeRunner(metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyNodeRunnerOptions) RunnerInterface {
	name := fmt.Sprintf("destroy.node.%s.%s.%s.%s", metaConfig.ProviderName, metaConfig.ClusterPrefix, metaConfig.Layout, opts.NodeGroupStep)

	return f.getOrCreateRunner(
		name,
		func() RunnerInterface {
			if f.provider == nil {
				panic("Executor provider must be set")
			}

			runner := NewRunnerFromConfig(metaConfig, opts.NodeGroupStep, stateCache, f.provider).
				WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, "")).
				WithName(opts.NodeName).
				WithAllowedCachedState(true)

			tomb.RegisterOnShutdown(opts.NodeName, runner.Stop)

			return applyAutomaticApproveSettings(runner, opts.AutoApproveSettings, f.stateChecker)
		},
	)
}
