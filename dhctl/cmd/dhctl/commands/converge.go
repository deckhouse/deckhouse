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

package commands

import (
	"fmt"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineConvergeCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		tmpDir := app.TmpDirName
		logger := log.GetDefaultLogger()
		isDebug := app.IsDebug

		var externalLogger *log.ExternalLogger
		if !app.DoNotWriteDebugLogFile {
			teeLogger, ok := logger.(*log.TeeLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to TeeLogger")
			}

			externalLogger, ok = teeLogger.GetLogger().(*log.ExternalLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to ExternalLogger")
			}
		} else {
			var ok bool
			externalLogger, ok = logger.(*log.ExternalLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to ExternalLogger")
			}
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: false,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissChanges:     false,
					AutoDismissDestructive: false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: false,
					},
				},
			},
			ProviderGetter:  providerGetter,
			TmpDir:          tmpDir,
			Logger:          logger,
			IsDebug:         isDebug,
			DirectoryConfig: app.GetDirConfig(),

			NoSwitchToNodeUser: app.ForceNoSwitchToNodeUser(),
		})
		converger.ApplyParams()
		_, err = converger.Converge(ctx)

		if err != nil {
			msg := fmt.Sprintf("Converge failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})
}

func DefineAutoConvergeCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineAutoConvergeFlags(cmd)
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		tmpDir := app.TmpDirName
		logger := log.GetDefaultLogger()
		isDebug := app.IsDebug

		var externalLogger *log.ExternalLogger
		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: true,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
			},
			ProviderGetter:  providerGetter,
			TmpDir:          tmpDir,
			Logger:          logger,
			IsDebug:         isDebug,
			DirectoryConfig: app.GetDirConfig(),
		})

		converger.ApplyParams()

		return converger.AutoConverge(ctx, app.AutoConvergeListenAddress, app.ApplyInterval)
	})
}

func DefineConvergeMigrationCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()
		var externalLogger *log.ExternalLogger

		if !app.DoNotWriteDebugLogFile {
			teeLogger, ok := logger.(*log.TeeLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to TeeLogger")
			}

			externalLogger, ok = teeLogger.GetLogger().(*log.ExternalLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to ExternalLogger")
			}
		} else {
			var ok bool
			externalLogger, ok = logger.(*log.ExternalLogger)
			if !ok {
				return fmt.Errorf("cannot convert logger to ExternalLogger")
			}
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		tmpDir := app.TmpDirName
		loggerFor := log.GetDefaultLogger()
		isDebug := app.IsDebug

		providersGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           loggerFor,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     true,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
				SkipChangesOnDeny: true,
			},
			CheckHasTerraformStateBeforeMigration: app.CheckHasTerraformStateBeforeMigrateToTofu,
			ProviderGetter:                        providersGetter,
			TmpDir:                                tmpDir,
			Logger:                                loggerFor,
			IsDebug:                               isDebug,
			DirectoryConfig:                       app.GetDirConfig(),
		})

		converger.ApplyParams()
		if err := converger.ConvergeMigration(ctx); err != nil {
			msg := fmt.Sprintf("ConvergeMigration failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})
}
