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

package main

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtologrus"
	"github.com/flant/kube-client/klogtologrus"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addon-operator/kube-config/backend"
	d8Apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	dhctl_commands "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	dhctl_app "github.com/deckhouse/deckhouse/dhctl/pkg/app"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis"
)

// Variables with component versions. They set by 'go build' command.
var (
	DeckhouseVersion     = "dev"
	AddonOperatorVersion = "dev"
	ShellOperatorVersion = "dev"
)

func version() string {
	return fmt.Sprintf("deckhouse %s (addon-operator %s, shell-operator %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion)
}

// main is almost a copy from addon-operator. We compile addon-operator to inline
// Go hooks and set some defaults. Also, helper commands are defined for Shell hooks.

const (
	AppName        = "deckhouse"
	AppDescription = "controller for Kubernetes platform from Flant"

	DefaultLogType         = "json"
	DefaultKubeClientQPS   = "20"
	DefaultKubeClientBurst = "40"
)

func main() {
	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion
	FileName := filepath.Base(os.Args[0])

	kpApp := kingpin.New(FileName, fmt.Sprintf("%s %s: %s", AppName, DeckhouseVersion, AppDescription))

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(sh_app.OperatorUsageTemplate(FileName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Println(version())
		return nil
	})

	kpApp.Action(func(c *kingpin.ParseContext) error {
		klogtologrus.InitAdapter(sh_app.DebugKubernetesAPI)
		stdliblogtologrus.InitAdapter()
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start deckhouse.").
		Action(func(c *kingpin.ParseContext) error {
			sh_app.AppStartMessage = version()

			ctx := context.Background()

			operator := addon_operator.NewAddonOperator(ctx)

			err := d8Apis.EnsureCRDs(ctx, operator.KubeClient(), "/deckhouse/deckhouse-controller/crds/*.yaml")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			operator.SetupKubeConfigManager(backend.New(operator.KubeClient().RestConfig(), nil))

			// TODO: remove deckhouse-config purge after release 1.56
			operator.ExplicitlyPurgeModules = []string{"deckhouse-config"}
			validation.RegisterAdmissionHandlers(operator)
			// TODO: move this routes to the deckhouse-controller
			module.SetupAdmissionRoutes(operator.AdmissionServer)

			err = operator.Setup()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			operator.ModuleManager.SetupModuleProducer(apis.NewModuleProducer())

			err = operator.Start()
			if err != nil {
				os.Exit(1)
			}

			// Init deckhouse-config service with ModuleManager instance.
			d8config.InitService(operator.ModuleManager)

			// Block main thread by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				operator.Stop()
				os.Exit(1)
			})

			return nil
		})
	// Set default log type as json
	sh_app.LogType = DefaultLogType
	sh_app.KubeClientQpsDefault = DefaultKubeClientQPS
	sh_app.KubeClientBurstDefault = DefaultKubeClientBurst
	ad_app.DefineStartCommandFlags(kpApp, startCmd)

	// Add debug commands from shell-operator and addon-operator
	sh_debug.DefineDebugCommands(kpApp)
	ad_app.DefineDebugCommands(kpApp)

	// Add more commands to the "module" command.
	debug.DefineModuleConfigDebugCommands(kpApp)

	// deckhouse-controller helper subcommands
	helpers.DefineHelperCommands(kpApp)

	// deckhouse-controller collect-debug-info
	debug.DefineCollectDebugInfoCommand(kpApp)

	// deckhouse-controller edit subcommands
	editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
	{
		dhctl_app.LoggerType = "json"
		dhctl_app.Editor = "vim"
		dhctl_app.KubeConfigInCluster = true
		dhctl_app.TmpDirName = os.TempDir()

		dhctl_commands.DefineEditCommands(editCmd /* wConnFlags */, false)
	}

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
