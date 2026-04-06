// Copyright 2021 Flant JSC
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

package bootstrap

import (
	"fmt"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineBootstrapInstallDeckhouseCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTFResourceManagementTimeout(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDeckhouseInstallFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		var sshClient node.SSHClient
		var err error

		if len(app.SSHHosts) != 0 {
			sshClient, err = sshclient.NewClientFromFlags(ctx)
			if err != nil {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			NodeInterface:   ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		return bootstraper.InstallDeckhouse(ctx)
	})
}

func DefineBootstrapExecuteBashibleCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlagsWithHosts(ctx)
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			NodeInterface:   ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		return bootstraper.ExecuteBashible(ctx)
	})
}

func DefineCreateResourcesCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineConfigsForResourcesPhaseFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineKubeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		var sshClient node.SSHClient
		var err error

		if len(app.SSHHosts) != 0 {
			sshClient, err = sshclient.NewClientFromFlags(ctx)
			if err != nil {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			NodeInterface:   ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		return bootstraper.CreateResources(ctx)
	})
}

func DefineBootstrapAbortCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineAbortFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlags(ctx)
		if err != nil {
			return err
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(logger.(*log.TeeLogger).GetLogger().(*log.ExternalLogger).GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			NodeInterface:          ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        app.GetDirConfig(),
		})

		if err = bootstraper.Abort(ctx, app.ForceAbortFromCache); err != nil {
			msg := fmt.Sprintf("Failed to abort cluster: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})
}

func DefineBaseInfrastructureCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		err := bootstraper.BaseInfrastructure(ctx)
		cache.GetGlobalTmpCleaner().DisableCleanup("Create base infra for cluster")
		return err
	})
}

func DefineExecPostBootstrapScript(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlagsWithHosts(ctx)
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			NodeInterface:   ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		return bootstraper.ExecPostBootstrap(ctx)
	})
}
