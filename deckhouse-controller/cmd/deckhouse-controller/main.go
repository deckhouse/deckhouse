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
	"path/filepath"

	addonapp "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtolog"
	"github.com/flant/kube-client/klogtolog"
	shellapp "github.com/flant/shell-operator/pkg/app"
	shelldebug "github.com/flant/shell-operator/pkg/debug"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/registry"
	dhctlcommands "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	dhctlapp "github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// main is almost a copy from addon-operator. We compile addon-operator to inline
// Go hooks and set some defaults. Also, helper commands are defined for Shell hooks.
func main() {
	shellapp.Version = app.VersionShellOperator
	addonapp.Version = app.VersionAddonOperator

	FileName := filepath.Base(os.Args[0])

	kpApp := kingpin.New(FileName, fmt.Sprintf("%s %s: %s", app.Name, app.VersionDeckhouse, app.Description))

	logger := log.NewLogger(log.Options{})
	log.SetDefault(logger)

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(shellapp.OperatorUsageTemplate(FileName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(_ *kingpin.ParseContext) error {
		fmt.Println(app.Version())
		return nil
	})

	kpApp.Action(func(_ *kingpin.ParseContext) error {
		klogtolog.InitAdapter(shellapp.DebugKubernetesAPI, logger.Named("klog"))
		stdliblogtolog.InitAdapter(logger)
		return nil
	})

	// start main loop
	startCmd := kpApp.
		Command("start", "Start deckhouse.").
		Action(start(logger))

	addonapp.DefineStartCommandFlags(kpApp, startCmd)

	// Add debug commands from shell-operator and addon-operator
	shelldebug.DefineDebugCommands(kpApp)
	addonapp.DefineDebugCommands(kpApp)

	// Add more commands to the "module" command.
	debug.DefineModuleConfigDebugCommands(kpApp, logger)

	// deckhouse-controller helper subcommands
	helpers.DefineHelperCommands(kpApp, logger)

	// deckhouse-controller collect-debug-info
	debug.DefineCollectDebugInfoCommand(kpApp)

	// deckhouse-controller requirements
	debug.DefineRequirementsCommands(kpApp)

	// deckhouse-controller registry
	registry.DefineRegistryCommand(kpApp, logger)

	// deckhouse-controller edit subcommands
	editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
	{
		dhctlapp.LoggerType = "json"
		dhctlapp.Editor = "vim"
		dhctlapp.KubeConfigInCluster = true
		dhctlapp.TmpDirName = os.TempDir()

		dhctlcommands.DefineEditCommands(editCmd /* wConnFlags */, false)
	}

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
