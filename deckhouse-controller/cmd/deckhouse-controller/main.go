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
	"fmt"
	_ "net/http/pprof"
	"os"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtologrus"
	"github.com/flant/kube-client/klogtologrus"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	dhctl_commands "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	dhctl_app "github.com/deckhouse/deckhouse/dhctl/pkg/app"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
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

	HookMetricsListenPort = "9651"
)

func main() {
	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion

	kpApp := kingpin.New(AppName, fmt.Sprintf("%s %s: %s", AppName, DeckhouseVersion, AppDescription))

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(sh_app.OperatorUsageTemplate(AppName))

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
			// Force separate port for hook metrics.
			if sh_app.HookMetricsListenPort == "" {
				sh_app.HookMetricsListenPort = HookMetricsListenPort
			}

			sh_app.AppStartMessage = version()

			// Workaround to run AddonOperator with deprecated settings:
			// - Init temporary Kubernetes client.
			// - Parse config from ConfigMap or load from ModuleConfig resources.
			// - Run conversions for sections.
			loader := d8config.NewInitialConfigLoader(nil)
			initialKubeConfig, err := loader.GetInitialKubeConfig(os.Getenv("ADDON_OPERATOR_CONFIG_MAP"))
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			operator := addon_operator.NewAddonOperator()
			operator.InitialKubeConfig = initialKubeConfig
			err = addon_operator.Bootstrap(operator)
			if err != nil {
				os.Exit(1)
			}
			operator.Start()

			// Init deckhouse-config service with ModuleManager instance.
			d8config.InitService(operator.ModuleManager)

			// Block main thread by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				operator.Shutdown()
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
