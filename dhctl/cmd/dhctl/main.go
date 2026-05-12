// Copyright 2026 Flant JSC
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
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry/kptelemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	oneShotDhctlServerCmd = "_server"
	grpcServerCmd         = "server"
	autoConvergeCmd       = "converge-periodical"
	terraformGroupCmd     = "terraform"
	exporterCmd           = "converge-exporter"
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
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "create-resources",
			Help:       "Create resources in Kubernetes cluster.",
			DefineFunc: bootstrap.DefineCreateResourcesCommand,
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "install-deckhouse",
			Help:       "Install deckhouse and wait for its readiness.",
			DefineFunc: bootstrap.DefineBootstrapInstallDeckhouseCommand,
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "abort",
			Help:       "Delete every node, which was created during bootstrap process.",
			DefineFunc: bootstrap.DefineBootstrapAbortCommand,
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "base-infra",
			Help:       "Create base infrastructure for Cloud Kubernetes cluster.",
			DefineFunc: bootstrap.DefineBaseInfrastructureCommand,
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "exec-post-bootstrap",
			Help:       "Test scp upload and ssh run uploaded script.",
			DefineFunc: bootstrap.DefineExecPostBootstrapScript,
			Parent:     "bootstrap-phase",
		},
		{
			Name:       "converge",
			Help:       "Converge kubernetes cluster.",
			DefineFunc: commands.DefineConvergeCommand,
		},
		{
			Name:       autoConvergeCmd,
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
			Parent:     "lock",
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
			Name: terraformGroupCmd,
			Help: "Infrastructure commands.",
		},
		{
			Name:       exporterCmd,
			Help:       "Run infrastructure converge exporter.",
			DefineFunc: commands.DefineInfrastructureConvergeExporterCommand,
			Parent:     "terraform",
		},
		{
			Name:       "check",
			Help:       "Check differences between state of Kubernetes cluster and infrastructure state.",
			DefineFunc: commands.DefineInfrastructureCheckCommand,
			Parent:     "terraform",
		},
		{
			Name: "config",
			Help: "Load, edit and save various dhctl configurations.",
		},
		{
			Name:   "parse",
			Help:   "Parse, validate and output configurations.",
			Parent: "config",
		},
		{
			Name:       "cluster-configuration",
			Help:       "Parse configuration and print it.",
			DefineFunc: commands.DefineCommandParseClusterConfiguration,
			Parent:     "parse",
		},
		{
			Name:       "cloud-discovery-data",
			Help:       "Parse cloud discovery data and print it.",
			DefineFunc: commands.DefineCommandParseCloudDiscoveryData,
			Parent:     "parse",
		},
		{
			Name:   "render",
			Help:   "Render transitional configurations.",
			Parent: "config",
		},
		{
			Name:       "bashible-bundle",
			Help:       "Render bashible bundle.",
			DefineFunc: commands.DefineRenderBashibleBundle,
			Parent:     "render",
		},
		{
			Name:       "control-plane-manifests",
			Help:       "Render control-plane manifests and pki.",
			DefineFunc: commands.DefineRenderControlPlaneAndPKI,
			Parent:     "render",
		},
		{
			Name:       "master-bootstrap-scripts",
			Help:       "Render master bootstrap scripts.",
			DefineFunc: commands.DefineRenderMasterBootstrap,
			Parent:     "render",
		},
		{
			Name:   "edit",
			Help:   "Change configuration files in Kubernetes cluster conveniently and safely.",
			Parent: "config",
			DefineFunc: func(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
				commands.DefineEditCommands(cmd, opts, true)
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
			Parent:     "test",
		},
		{
			Name:       "kubernetes-api-connection",
			Help:       "Test connection to kubernetes api via ssh or directly.",
			DefineFunc: commands.DefineTestKubernetesAPIConnectionCommand,
			Parent:     "test",
		},
		{
			Name:       "scp",
			Help:       "Test scp file operations.",
			DefineFunc: commands.DefineTestSCPCommand,
			Parent:     "test",
		},
		{
			Name:       "upload-exec",
			Help:       "Test scp upload and ssh run uploaded script.",
			DefineFunc: commands.DefineTestUploadExecCommand,
			Parent:     "test",
		},
		{
			Name:       "bashible-bundle",
			Help:       "Test upload and execute a bundle.",
			DefineFunc: commands.DefineTestBundle,
			Parent:     "test",
		},
		{
			Name:   "control-plane",
			Help:   "Commands to test control plane nodes.",
			Parent: "test",
		},
		{
			Name:       "manager",
			Help:       "Test control plane manager is ready.",
			DefineFunc: commands.DefineTestControlPlaneManagerReadyCommand,
			Parent:     "control-plane",
		},
		{
			Name:       "node",
			Help:       "Test control plane node is ready.",
			DefineFunc: commands.DefineTestControlPlaneNodeReadyCommand,
			Parent:     "control-plane",
		},
		{
			Name:   "deckhouse",
			Help:   "Install and uninstall deckhouse.",
			Parent: "test",
		},
		{
			Name:       "create-deployment",
			Help:       "Install deckhouse after infrastructure is applied successful.",
			DefineFunc: commands.DefineDeckhouseCreateDeployment,
			Parent:     "deckhouse",
		},
		{
			Name:       "remove-deployment",
			Help:       "Delete deckhouse deployment.",
			DefineFunc: commands.DefineDeckhouseRemoveDeployment,
			Parent:     "deckhouse",
		},
		{
			Name:       "deployment-ready",
			Help:       "Wait while deployment is ready.",
			DefineFunc: commands.DefineWaitDeploymentReadyCommand,
			Parent:     "deckhouse",
		},
	}
)

func registerOnShutdown(title string, action onShutdownFunc) {
	tomb.RegisterOnShutdown(title, action)
}

func main() {
	appContext := context.Background()

	opts := options.New()

	initGlobalVars()

	if err := telemetry.Bootstrap(appContext); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	appContext, applicationSpan := telemetry.StartApplication(appContext)
	tomb.RegisterOnShutdown("End telemetry application span", func() { applicationSpan.End() })

	registerOnShutdown("Restore terminal if needed", restoreTerminal())
	registerOnShutdown("Stop default SSH session", process.DefaultSession.Stop)

	go tomb.WaitForProcessInterruption(tomb.BeforeInterrupted{
		disableCleanupOnInterrupted,
	})

	kpApp := kingpin.New(app.AppName, "A tool to create Kubernetes cluster and infrastructure.")
	kpApp.HelpFlag.Short('h')
	app.GlobalFlags(kpApp, &opts.Global)

	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, opts.BuildInfo.AppVersion)
		return nil
	})

	if err := registerCommands(kpApp, opts); err != nil {
		panic(err)
	}

	runApplication(appContext, kpApp, opts)
}

func runApplication(ctx context.Context, kpApp *kingpin.Application, opts *options.Options) {
	initer := newActionIniter(opts)

	// inject context.Context to kingpin.ParseContext
	kpApp.Action(kpcontext.SetContextToAction(ctx))

	kpApp.Action(kptelemetry.StartCommand)

	kpApp.Action(func(c *kingpin.ParseContext) error {
		initer.setParams(actionIniterParams{
			tmpDirName:        opts.Global.TmpDir,
			stateCacheDirName: opts.Cache.Dir,

			isDebug: opts.Global.IsDebug,

			loggerType:          opts.Global.LoggerType,
			doNotWriteDebugFile: opts.Global.DoNotWriteDebugLogFile,
			debugLogFilePath:    opts.Global.DebugLogFilePath,
		})

		initer.setRegisterOnShutdown(registerOnShutdown)

		return initer.init(c)
	})

	kpApp.Version(opts.BuildInfo.AppVersion).Author("Flant")

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
		kptelemetry.EndCommand(err, errorCode)
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
	template.InitGlobalVars(dhctlPath)
	infrastructure.InitGlobalVars(dhctlPath)
}
