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
	"path"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"strings"
	"time"

	terminal "golang.org/x/term"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var (
	allowedCommands []string
)

const (
	serverCommand                           = "server"
	singleThreadedServerCommand             = "_server"
	bootstrapCommand                        = "bootstrap"
	bootstrapPhaseCommand                   = "bootstrap-phase"
	executeBashibleSubcommand               = "execute-bashible-bundle"
	installDeckhouseSubcommand              = "install-deckhouse"
	createResourcesSubcommand               = "create-resources"
	abortSubcommand                         = "abort"
	baseInfrastructureSubcommand            = "base-infra"
	execPostBootstarpSubcommand             = "exec-post-bootstrap"
	convergeCommand                         = "converge"
	autoConvergeCommand                     = "converge-periodical"
	lockCommand                             = "lock"
	lockReleaseSubcommand                   = "release"
	destroyCommand                          = "destroy"
	terraformCommand                        = "terraform"
	convergeExporterSubcommand              = "converge-exporter"
	terraformCheckSubcommand                = "check"
	configCommand                           = "comfig"
	parseSubcommand                         = "parse"
	renderSubcommand                        = "render"
	editSubcommand                          = "edit"
	parseClusterConfigurationSubcommand     = "cluster-configuration"
	parseCloudDiscoveryDataSubcommand       = "cloud-discovery-data"
	renderBashibleBundleSubcommand          = "bashible-bundle"
	renderKubeadmConfigSubcommand           = "kubeadm-config"
	renderMasterBootstrapSubcommand         = "master-bootstrap-scripts"
	testCommand                             = "test"
	testSSHConnectionSubcommand             = "ssh-connection"
	testKubernetesAPIConnectionSubcommand   = "kubernetes-api-connection"
	testSCPSubcommand                       = "scp"
	testUploadExecSubcommand                = "upload-exec"
	testBundleSubcommand                    = "bashible-bundle"
	testControlPlaneSubcommand              = "control-plane"
	testControlPlaneManagerSubcommand       = "manager"
	testControlPlaneNodeSubcommand          = "node"
	testDeckhouseSubcommand                 = "deckhouse"
	testDeckhouseCreateDeploymentSubcommand = "create-deployment"
	testDeckhouseRemoveDeploymentSubcommand = "remove-deployment"
	testWaitDeploymentReadySubcommand       = "deployment-ready"
)

func main() {
	_ = os.Mkdir(app.TmpDirName, 0o755)

	initGlobalVars()

	tomb.RegisterOnShutdown("Trace", EnableTrace())
	tomb.RegisterOnShutdown("Restore terminal if needed", restoreTerminal())
	tomb.RegisterOnShutdown("Stop default SSH session", process.DefaultSession.Stop)
	tomb.RegisterOnShutdown("Clear dhctl temporary directory", cache.ClearTemporaryDirs)
	tomb.RegisterOnShutdown("Clear terraform data temporary directory", cache.ClearTerraformDir)

	go tomb.WaitForProcessInterruption()

	kpApp := kingpin.New(app.AppName, "A tool to create Kubernetes cluster and infrastructure.")
	kpApp.HelpFlag.Short('h')
	app.GlobalFlags(kpApp)

	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	allowed, _ := checkCommand(serverCommand, allowedCommands)
	if allowed {
		commands.DefineServerCommand(kpApp, serverCommand)
	}

	allowed, _ = checkCommand(singleThreadedServerCommand, allowedCommands)
	if allowed {
		commands.DefineSingleThreadedServerCommand(kpApp, singleThreadedServerCommand)
	}

	allowed, _ = checkCommand(bootstrapCommand, allowedCommands)
	if allowed {
		bootstrap.DefineBootstrapCommand(kpApp, bootstrapCommand)
	}

	allowed, subcommands := checkCommand(bootstrapPhaseCommand, allowedCommands)
	if allowed {
		bootstrapPhaseCmd := kpApp.Command(bootstrapPhaseCommand, "Commands to run a single phase of the bootstrap process.")
		{
			if checkSubcommand(executeBashibleSubcommand, subcommands) {
				bootstrap.DefineBootstrapExecuteBashibleCommand(bootstrapPhaseCmd, executeBashibleSubcommand)
			}

			if checkSubcommand(installDeckhouseSubcommand, subcommands) {
				bootstrap.DefineBootstrapInstallDeckhouseCommand(bootstrapPhaseCmd, installDeckhouseSubcommand)
			}

			if checkSubcommand(createResourcesSubcommand, subcommands) {
				bootstrap.DefineCreateResourcesCommand(bootstrapPhaseCmd, createResourcesSubcommand)
			}

			if checkSubcommand(abortSubcommand, subcommands) {
				bootstrap.DefineBootstrapAbortCommand(bootstrapPhaseCmd, abortSubcommand)
			}

			if checkSubcommand(baseInfrastructureSubcommand, subcommands) {
				bootstrap.DefineBaseInfrastructureCommand(bootstrapPhaseCmd, baseInfrastructureSubcommand)
			}

			if checkSubcommand(execPostBootstarpSubcommand, subcommands) {
				bootstrap.DefineExecPostBootstrapScript(bootstrapPhaseCmd, execPostBootstarpSubcommand)
			}
		}
	}

	allowed, _ = checkCommand(convergeCommand, allowedCommands)
	if allowed {
		commands.DefineConvergeCommand(kpApp, convergeCommand)
	}

	allowed, _ = checkCommand(autoConvergeCommand, allowedCommands)
	if allowed {
		commands.DefineAutoConvergeCommand(kpApp, autoConvergeCommand)
	}

	allowed, _ = checkCommand(lockCommand, allowedCommands)
	if allowed {
		lockCmd := kpApp.Command(lockCommand, "Converge cluster lock")
		{
			commands.DefineReleaseConvergeLockCommand(lockCmd, lockReleaseSubcommand)
		}
	}

	allowed, _ = checkCommand(destroyCommand, allowedCommands)
	if allowed {
		commands.DefineDestroyCommand(kpApp, destroyCommand)
	}

	allowed, subcommands = checkCommand(terraformCommand, allowedCommands)
	if allowed {
		terraformCmd := kpApp.Command(terraformCommand, "Terraform commands.")
		{
			if checkSubcommand(convergeExporterSubcommand, subcommands) {
				commands.DefineTerraformConvergeExporterCommand(terraformCmd, convergeExporterSubcommand)
			}

			if checkSubcommand(terraformCheckSubcommand, subcommands) {
				commands.DefineTerraformCheckCommand(terraformCmd, terraformCheckSubcommand)
			}
		}
	}

	allowed, subcommands = checkCommand(configCommand, allowedCommands)
	if allowed {
		configCmd := kpApp.Command(configCommand, "Load, edit and save various dhctl configurations.")
		{
			if checkSubcommand(parseSubcommand, subcommands) {
				parseCmd := configCmd.Command(parseSubcommand, "Parse, validate and output configurations.")
				{
					commands.DefineCommandParseClusterConfiguration(kpApp, parseCmd, parseClusterConfigurationSubcommand)
					commands.DefineCommandParseCloudDiscoveryData(kpApp, parseCmd, parseCloudDiscoveryDataSubcommand)
				}
			}

			if checkSubcommand(renderSubcommand, subcommands) {
				renderCmd := configCmd.Command(renderSubcommand, "Render transitional configurations.")
				{
					commands.DefineRenderBashibleBundle(renderCmd, renderBashibleBundleSubcommand)
					commands.DefineRenderKubeadmConfig(renderCmd, renderKubeadmConfigSubcommand)
					commands.DefineRenderMasterBootstrap(renderCmd, renderMasterBootstrapSubcommand)
				}
			}

			if checkSubcommand(editSubcommand, subcommands) {
				editCmd := configCmd.Command(editSubcommand, "Change configuration files in Kubernetes cluster conveniently and safely.")
				{
					commands.DefineEditCommands(editCmd /* wConnFlags */, true)
				}
			}
		}
	}

	allowed, subcommands = checkCommand(testCommand, allowedCommands)
	if allowed {
		testCmd := kpApp.Command(testCommand, "Commands to test the parts of bootstrap and converge process.")
		{
			if checkSubcommand(testSSHConnectionSubcommand, subcommands) {
				commands.DefineTestSSHConnectionCommand(testCmd, testSSHConnectionSubcommand)
			}

			if checkSubcommand(testKubernetesAPIConnectionSubcommand, subcommands) {
				commands.DefineTestKubernetesAPIConnectionCommand(testCmd, testKubernetesAPIConnectionSubcommand)
			}

			if checkSubcommand(testSCPSubcommand, subcommands) {
				commands.DefineTestSCPCommand(testCmd, testSCPSubcommand)
			}

			if checkSubcommand(testUploadExecSubcommand, subcommands) {
				commands.DefineTestUploadExecCommand(testCmd, testUploadExecSubcommand)
			}

			if checkSubcommand(testBundleSubcommand, subcommands) {
				commands.DefineTestBundle(testCmd, testBundleSubcommand)
			}

			if checkSubcommand(testControlPlaneSubcommand, subcommands) {
				controlPlaneCmd := testCmd.Command(testControlPlaneSubcommand, "Commands to test control plane nodes.")
				{
					commands.DefineTestControlPlaneManagerReadyCommand(controlPlaneCmd, testControlPlaneManagerSubcommand)
					commands.DefineTestControlPlaneNodeReadyCommand(controlPlaneCmd, testControlPlaneNodeSubcommand)
				}
			}
		}

		if checkSubcommand(testDeckhouseSubcommand, subcommands) {
			deckhouseCmd := testCmd.Command(testDeckhouseSubcommand, "Install and uninstall deckhouse.")
			{
				commands.DefineDeckhouseCreateDeployment(deckhouseCmd, testDeckhouseCreateDeploymentSubcommand)
				commands.DefineDeckhouseRemoveDeployment(deckhouseCmd, testDeckhouseRemoveDeploymentSubcommand)
				commands.DefineWaitDeploymentReadyCommand(deckhouseCmd, testWaitDeploymentReadySubcommand)
			}
		}
	}

	runApplication(kpApp)
}

func runApplication(kpApp *kingpin.Application) {
	kpApp.Action(func(c *kingpin.ParseContext) error {
		log.InitLogger(app.LoggerType)
		if app.DoNotWriteDebugLogFile {
			return nil
		}

		if c.SelectedCommand == nil {
			return nil
		}

		logPath := app.DebugLogFilePath

		if logPath == "" {
			cmdStr := strings.Join(strings.Fields(c.SelectedCommand.FullCommand()), "")
			logFile := cmdStr + "-" + time.Now().Format("20060102150405") + ".log"
			logPath = path.Join(app.TmpDirName, logFile)
		}

		outFile, err := os.Create(logPath)
		if err != nil {
			return err
		}

		err = log.WrapWithTeeLogger(outFile, 1024)
		if err != nil {
			return err
		}

		log.InfoF("Debug log file: %s\n", logPath)

		tomb.RegisterOnShutdown("Finalize logger", func() {
			if err := log.FlushAndClose(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to flush and close log file: %v\n", err)
				return
			}
		})

		return nil
	})

	kpApp.Version(app.AppVersion).Author("Flant")

	go func() {
		command, err := kpApp.Parse(os.Args[1:])
		errorCode := 0
		if err != nil {
			log.DebugLn(command)
			log.ErrorLn(err)
			errorCode = 1
		}
		tomb.Shutdown(errorCode)
	}()

	// Block "main" function until teardown callbacks are finished.
	exitCode := tomb.WaitShutdown()
	os.Exit(exitCode)
}

func EnableTrace() func() {
	traceFileName := os.Getenv("DHCTL_TRACE")
	cpuProfileFileName := traceFileName + ".prof.cpu"

	if traceFileName == "" || traceFileName == "0" || traceFileName == "no" {
		return func() {}
	}
	if traceFileName == "1" || traceFileName == "yes" {
		traceFileName = "trace.out"
		cpuProfileFileName = "pprof.cpu"
	}

	fns := make([]func(), 0)

	traceF, err := os.Create(traceFileName)
	if err != nil {
		log.InfoF("failed to create trace output file '%s': %v", traceFileName, err)
		os.Exit(1)
	}

	fns = append([]func(){
		func() {
			if err := traceF.Close(); err != nil {
				log.InfoF("failed to close trace file '%s': %v", traceFileName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	profCPU, err := os.Create(cpuProfileFileName)
	if err != nil {
		log.InfoF("failed to create pprof cpu file '%s': %v", cpuProfileFileName, err)
		os.Exit(1)
	}

	fns = append([]func(){
		func() {
			if err := profCPU.Close(); err != nil {
				log.InfoF("failed to close pprof cpu file '%s': %v", cpuProfileFileName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	if err := trace.Start(traceF); err != nil {
		log.InfoF("failed to start trace to '%s': %v", traceFileName, err)
		os.Exit(1)
	}
	fns = append([]func(){
		trace.Stop,
	}, fns...)

	if err := pprof.StartCPUProfile(profCPU); err != nil {
		log.InfoF("failed to start profile cpu to '%s': %v", cpuProfileFileName, err)
		os.Exit(1)
	}

	fns = append([]func(){
		pprof.StopCPUProfile,
	}, fns...)

	return func() {
		for _, fn := range fns {
			fn()
		}
	}
}

func restoreTerminal() func() {
	fd := int(os.Stdin.Fd())
	if !terminal.IsTerminal(fd) {
		return func() {}
	}

	state, err := terminal.GetState(fd)
	if err != nil {
		panic(err)
	}

	return func() { _ = terminal.Restore(fd, state) }
}

func initGlobalVars() {
	// get current location of called binary
	dhctlPath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
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

	commandsEnv := os.Getenv("DHCTL_CLI_ALLOWED_COMMANDS")

	if len(commandsEnv) > 0 {
		allowedCommands = strings.Split(commandsEnv, ", ")
	}

	// set relative path to config and template files
	config.InitGlobalVars(dhctlPath)
	commands.InitGlobalVars(dhctlPath)
	app.InitGlobalVars(dhctlPath)
	terraform.InitGlobalVars(dhctlPath)
	manifests.InitGlobalVars(dhctlPath)
	template.InitGlobalVars(dhctlPath)
}

func checkCommand(name string, allowedCommands []string) (bool, []string) {
	if len(allowedCommands) == 0 || slices.Index(allowedCommands, name) != -1 {
		return true, []string{}
	}

	for _, cm := range allowedCommands {
		c := strings.Split(cm, " ")
		if c[0] == name {
			return true, c
		}
	}

	return false, []string{}
}

func checkSubcommand(name string, subcommands []string) bool {
	ex, _ := checkCommand(name, subcommands)
	if len(subcommands) == 2 && subcommands[1] == "*" || ex {
		return true
	}

	return false
}
