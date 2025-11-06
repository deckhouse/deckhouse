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
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	oneShotDhctlServerCmd = "_server"
	grpcServerCmd         = "server"
)

var (
	commandList = []Command{
		{
			Name:       grpcServerCmd,
			Help:       "Start dhctl as GRPC server.",
			DefineFunc: commands.DefineServerCommand,
		},
		{
			Name:       oneShotDhctlServerCmd,
			Help:       "Start dhctl as GRPC server. Single threaded version.",
			DefineFunc: commands.DefineSingleThreadedServerCommand,
		},
		{
			Name:       "bootstrap",
			Help:       "Bootstrap cluster.",
			DefineFunc: bootstrap.DefineBootstrapCommand,
		},
		{
			Name: "bootstrap-phase",
			Help: "Commands to run a single phase of the bootstrap process.",
		},
		{
			Name:       "execute-bashible-bundle",
			Help:       "Prepare Master node and install Kubernetes.",
			DefineFunc: bootstrap.DefineBootstrapExecuteBashibleCommand,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "create-resources",
			Help:       "Create resources in Kubernetes cluster.",
			DefineFunc: bootstrap.DefineCreateResourcesCommand,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "install-deckhouse",
			Help:       "Install deckhouse and wait for its readiness.",
			DefineFunc: bootstrap.DefineBootstrapInstallDeckhouseCommand,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "abort",
			Help:       "Delete every node, which was created during bootstrap process.",
			DefineFunc: bootstrap.DefineBootstrapAbortCommand,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "base-infra",
			Help:       "Create base infrastructure for Cloud Kubernetes cluster.",
			DefineFunc: bootstrap.DefineBaseInfrastructureCommand,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "exec-post-bootstrap",
			Help:       "Test scp upload and ssh run uploaded script.",
			DefineFunc: bootstrap.DefineExecPostBootstrapScript,
			Parrent:    "bootstrap-phase",
		},
		{
			Name:       "converge",
			Help:       "Converge kubernetes cluster.",
			DefineFunc: commands.DefineConvergeCommand,
		},
		{
			Name:       "converge-periodical",
			Help:       "Start service for periodical run converge.",
			DefineFunc: commands.DefineAutoConvergeCommand,
		},
		{
			Name:       "converge-migration",
			Help:       "Migrate state from terraform to opentofu. Starting converge if cluster has not infrastructure changes.",
			DefineFunc: commands.DefineConvergeMigrationCommand,
		},
		{
			Name: "lock",
			Help: "Converge cluster lock",
		},
		{
			Name:       "release",
			Help:       "Release converge lock fully. It's remove converge lease lock from cluster regardless of owner. Be careful",
			DefineFunc: commands.DefineReleaseConvergeLockCommand,
			Parrent:    "lock",
		},
		{
			Name:       "destroy",
			Help:       "Destroy Kubernetes cluster.",
			DefineFunc: commands.DefineDestroyCommand,
		},
		{
			Name:       "session",
			Help:       "SSH tunnel proxy to Kubernetes cluster and save local kubeconfig for kubectl.",
			DefineFunc: commands.DefineSessionCommand,
		},
		{
			Name: "terraform",
			Help: "Infrastructure commands.",
		},
		{
			Name:       "converge-exporter",
			Help:       "Run infrastructure converge exporter.",
			DefineFunc: commands.DefineInfrastructureConvergeExporterCommand,
			Parrent:    "terraform",
		},
		{
			Name:       "check",
			Help:       "Check differences between state of Kubernetes cluster and infrastructure state.",
			DefineFunc: commands.DefineInfrastructureCheckCommand,
			Parrent:    "terraform",
		},
		{
			Name: "config",
			Help: "Load, edit and save various dhctl configurations.",
		},
		{
			Name:    "parse",
			Help:    "Parse, validate and output configurations.",
			Parrent: "config",
		},
		{
			Name:       "cluster-configuration",
			Help:       "Parse configuration and print it.",
			DefineFunc: commands.DefineCommandParseClusterConfiguration,
			Parrent:    "parse",
		},
		{
			Name:       "cloud-discovery-data",
			Help:       "Parse cloud discovery data and print it.",
			DefineFunc: commands.DefineCommandParseCloudDiscoveryData,
			Parrent:    "parse",
		},
		{
			Name:    "render",
			Help:    "Render transitional configurations.",
			Parrent: "config",
		},
		{
			Name:       "bashible-bundle",
			Help:       "Render bashible bundle.",
			DefineFunc: commands.DefineRenderBashibleBundle,
			Parrent:    "render",
		},
		{
			Name:       "kubeadm-config",
			Help:       "Render kubeadm config.",
			DefineFunc: commands.DefineRenderKubeadmConfig,
			Parrent:    "render",
		},
		{
			Name:       "master-bootstrap-scripts",
			Help:       "Render master bootstrap scripts.",
			DefineFunc: commands.DefineRenderMasterBootstrap,
			Parrent:    "render",
		},
		{
			Name:    "edit",
			Help:    "Change configuration files in Kubernetes cluster conveniently and safely.",
			Parrent: "config",
			DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause {
				commands.DefineEditCommands(cmd /* wConnFlags */, true)
				return nil
			},
		},
		{
			Name: "test",
			Help: "Commands to test the parts of bootstrap and converge process.",
		},
		{
			Name:       "ssh-connection",
			Help:       "Test connection via ssh.",
			DefineFunc: commands.DefineTestSSHConnectionCommand,
			Parrent:    "test",
		},
		{
			Name:       "kubernetes-api-connection",
			Help:       "Test connection to kubernetes api via ssh or directly.",
			DefineFunc: commands.DefineTestKubernetesAPIConnectionCommand,
			Parrent:    "test",
		},
		{
			Name:       "scp",
			Help:       "Test scp file operations.",
			DefineFunc: commands.DefineTestSCPCommand,
			Parrent:    "test",
		},
		{
			Name:       "upload-exec",
			Help:       "Test scp upload and ssh run uploaded script.",
			DefineFunc: commands.DefineTestUploadExecCommand,
			Parrent:    "test",
		},
		{
			Name:       "bashible-bundle",
			Help:       "Test upload and execute a bundle.",
			DefineFunc: commands.DefineTestBundle,
			Parrent:    "test",
		},
		{
			Name:    "control-plane",
			Help:    "Commands to test control plane nodes.",
			Parrent: "test",
		},
		{
			Name:       "manager",
			Help:       "Test control plane manager is ready.",
			DefineFunc: commands.DefineTestControlPlaneManagerReadyCommand,
			Parrent:    "control-plane",
		},
		{
			Name:       "node",
			Help:       "Test control plane node is ready.",
			DefineFunc: commands.DefineTestControlPlaneNodeReadyCommand,
			Parrent:    "control-plane",
		},
		{
			Name:    "deckhouse",
			Help:    "Install and uninstall deckhouse.",
			Parrent: "test",
		},
		{
			Name:       "create-deployment",
			Help:       "Install deckhouse after infrastructure is applied successful.",
			DefineFunc: commands.DefineDeckhouseCreateDeployment,
			Parrent:    "deckhouse",
		},
		{
			Name:       "remove-deployment",
			Help:       "Delete deckhouse deployment.",
			DefineFunc: commands.DefineDeckhouseRemoveDeployment,
			Parrent:    "deckhouse",
		},
		{
			Name:       "deployment-ready",
			Help:       "Wait while deployment is ready.",
			DefineFunc: commands.DefineWaitDeploymentReadyCommand,
			Parrent:    "deckhouse",
		},
	}
)

func registerOnShutdown(title string, action func()) {
	tomb.RegisterOnShutdown(title, action)
}

func main() {
	initGlobalVars()

	tracesShutdownFn, err := enableTrace()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	registerOnShutdown("Trace", tracesShutdownFn)
	registerOnShutdown("Restore terminal if needed", restoreTerminal())
	registerOnShutdown("Stop default SSH session", process.DefaultSession.Stop)

	go tomb.WaitForProcessInterruption()

	kpApp := kingpin.New(app.AppName, "A tool to create Kubernetes cluster and infrastructure.")
	kpApp.HelpFlag.Short('h')
	app.GlobalFlags(kpApp)

	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	if err := registerCommands(kpApp); err != nil {
		panic(err)
	}

	runApplication(kpApp)
}

func runApplication(kpApp *kingpin.Application) {
	initer := newActionIniter()

	kpApp.Action(func(c *kingpin.ParseContext) error {
		initer.setParams(actionIniterParams{
			tmpDirName: app.TmpDirName,
			isDebug:    app.IsDebug,

			loggerType:          app.LoggerType,
			doNotWriteDebugFile: app.DoNotWriteDebugLogFile,
			debugLogFilePath:    app.DebugLogFilePath,
		})

		initer.setRegisterOnShutdown(registerOnShutdown)

		return initer.init(c)
	})

	kpApp.Version(app.AppVersion).Author("Flant")

	go func() {
		command, err := kpApp.Parse(os.Args[1:])
		errorCode := 0
		if err != nil {
			log.DebugLn(command)

			msg := err.Error()

			if logFile := initer.getLoggerPath(); logFile != "" {
				msg = fmt.Sprintf("%s\nDebug log file: %s", msg, logFile)
			}

			log.ErrorLn(msg)
			errorCode = 1
		}
		tomb.Shutdown(errorCode)
	}()

	// Block "main" function until teardown callbacks are finished.
	exitCode := tomb.WaitShutdown()
	os.Exit(exitCode)
}

func initGlobalVars() {
	dhctlPath := ""

	if val, ok := os.LookupEnv("DHCTL_SKIP_LOOKUP_EXEC_PATH"); !ok || val != "yes" {
		// get current location of called binary
		var err error
		dhctlPath, err = os.Readlink(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
		if err != nil {
			panic(err)
		}
		dhctlPath = filepath.Dir(dhctlPath)
		if dhctlPath == "/" {
			dhctlPath = "" // All our paths are already absolute by themselves
		}

		// set path to ssh and terraform binaries
		if err = os.Setenv("PATH", fmt.Sprintf("%s/bin:%s", dhctlPath, os.Getenv("PATH"))); err != nil {
			panic(err)
		}
	}

	commandsEnv := os.Getenv("DHCTL_CLI_ALLOWED_COMMANDS")

	if len(commandsEnv) > 0 {
		allowedCommands = strings.Split(commandsEnv, ", ")
	}

	// set relative path to config and template files
	config.InitGlobalVars(dhctlPath)
	commands.InitGlobalVars(dhctlPath)
	app.InitGlobalVars(dhctlPath)
	manifests.InitGlobalVars(dhctlPath)
	template.InitGlobalVars(dhctlPath)
	infrastructure.InitGlobalVars(dhctlPath)
}
