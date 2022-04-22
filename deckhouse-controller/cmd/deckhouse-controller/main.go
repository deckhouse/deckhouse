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

	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtologrus"
	"github.com/flant/kube-client/klogtologrus"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/config"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/deckhouse"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
)

// Variables with component versions. They set by 'go build' command.
var (
	DeckhouseVersion     = "dev"
	AddonOperatorVersion = "dev"
	ShellOperatorVersion = "dev"
)

func main() {
	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion

	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, DeckhouseVersion, app.AppDescription))

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(sh_app.OperatorUsageTemplate(app.AppName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("deckhouse %s (addon-operator %s, shell-operator %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion)
		return nil
	})

	kpApp.Action(func(c *kingpin.ParseContext) error {
		klogtologrus.InitAdapter(sh_app.DebugKubernetesAPI)
		stdliblogtologrus.InitAdapter()
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start deckhouse.").
		Default().
		Action(func(c *kingpin.ParseContext) error {
			runtimeConfig := config.NewConfig()
			// Init logging subsystem.
			sh_app.SetupLogging(runtimeConfig)
			log.Infof("deckhouse %s (addon-operator %s, shell-operator %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion)

			// Set hook metrics listen port if flat is not passed.
			if sh_app.HookMetricsListenPort == "" {
				sh_app.HookMetricsListenPort = app.DeckhouseHookMetricsListenPort
			}

			operator := deckhouse.DefaultDeckhouse()
			operator.WithRuntimeConfig(runtimeConfig)
			err := deckhouse.InitAndStart(operator)
			if err != nil {
				os.Exit(1)
			}

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				operator.Shutdown()
				os.Exit(1)
			})

			return nil
		})
	// Set default log type as json
	sh_app.LogType = app.DeckhouseLogTypeDefault
	sh_app.KubeClientQpsDefault = app.DeckhouseKubeClientQPSDefault
	sh_app.KubeClientBurstDefault = app.DeckhouseKubeClientBurstDefault
	app.DefineStartCommandFlags(startCmd)
	ad_app.DefineStartCommandFlags(kpApp, startCmd)

	// Add debug commands from shell-operator and addon-operator
	sh_debug.DefineDebugCommands(kpApp)
	ad_app.DefineDebugCommands(kpApp)

	// deckhouse-controller helper subcommands
	helpers.DefineHelperCommands(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
