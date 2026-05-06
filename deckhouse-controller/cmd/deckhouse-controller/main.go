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
	"runtime"

	ad_app "github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/utils/stdliblogtolog"
	"github.com/flant/kube-client/klogtolog"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/dhctlcli"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Variables with component versions. They set by 'go build' command.
var (
	DeckhouseVersion     = "dev"
	AddonOperatorVersion = "dev"
	ShellOperatorVersion = "dev"
	NelmVersion          = "dev"
)

// Variables to configure with build flags.
var (
	DefaultReleaseChannel = ""
)

const (
	defaultReleaseChannel = "Stable"
)

func version() string {
	return fmt.Sprintf("deckhouse %s (addon-operator %s, shell-operator %s, nelm %s, Golang %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion, NelmVersion, runtime.Version())
}

// main is almost a copy from addon-operator. We compile addon-operator to inline
// Go hooks and set some defaults. Also, helper commands are defined for Shell hooks.

const (
	AppName        = "deckhouse"
	AppDescription = "controller for Kubernetes platform from Flant"
)

func main() {
	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion

	FileName := filepath.Base(os.Args[0])

	kpApp := kingpin.New(FileName, fmt.Sprintf("%s %s: %s", AppName, DeckhouseVersion, AppDescription))

	logger := log.NewLogger()
	log.SetDefault(logger)

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(sh_app.OperatorUsageTemplate(FileName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(_ *kingpin.ParseContext) error {
		fmt.Println(version())
		return nil
	})

	kpApp.Action(func(_ *kingpin.ParseContext) error {
		klogtolog.InitAdapter(sh_app.DebugKubernetesAPI, logger.Named("klog"))
		stdliblogtolog.InitAdapter(logger)
		return nil
	})

	// start main loop
	startCmd := kpApp.
		Command("start", "Start deckhouse.").
		Action(start(logger))

	ad_app.DefineStartCommandFlags(kpApp, startCmd)

	// Add debug commands from shell-operator and addon-operator
	sh_debug.DefineDebugCommands(kpApp)
	ad_app.DefineDebugCommands(kpApp)

	// Add more commands to the "module" command.
	debug.DefineModuleConfigDebugCommands(kpApp, logger)

	// deckhouse-controller helper subcommands
	helpers.DefineHelperCommands(kpApp, logger)

	// deckhouse-controller requirements
	debug.DefineRequirementsCommands(kpApp)

	// deckhouse-controller packages
	debug.DefinePackagesCommands(kpApp)

	// deckhouse-controller registry
	registry.DefineRegistryCommand(kpApp, logger)

	// dhctlcli command builders read defaults from DHCTL_CLI_* env vars
	// (kingpin Envar bindings in dhctl/pkg/app); seed them here so we don't
	// import dhctl/pkg/app from main. Deployer-set values are preserved.
	{
		setDhctlEnvDefault("DHCTL_CLI_LOGGER_TYPE", "json")
		setDhctlEnvDefault("DHCTL_CLI_EDITOR", "vim")
		setDhctlEnvDefault("DHCTL_CLI_KUBE_CLIENT_FROM_CLUSTER", "true")
		setDhctlEnvDefault("DHCTL_CLI_TMP_DIR", os.TempDir())

		editCmd := kpApp.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
		dhctlcli.DefineEditCommands(editCmd /* wConnFlags */, false)

		dhctlcli.DefineCommandParseClusterConfiguration(kpApp.Command("cluster-configuration", "Parse configuration and print it."))
		dhctlcli.DefineCommandParseCloudDiscoveryData(kpApp.Command("cloud-discovery-data", "Parse cloud discovery data and print it."))
	}

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}

// setDhctlEnvDefault seeds an env var only if the deployer hasn't set one.
// os.Setenv error is dropped: it only fails on names containing '=' or NUL.
func setDhctlEnvDefault(name, value string) {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return
	}
	_ = os.Setenv(name, value)
}
