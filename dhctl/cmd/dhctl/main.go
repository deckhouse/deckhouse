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
	allowedCommands                         []string
	bootstrapCommand                        = Command{Name: "bootstrap", Help: "Bootstrap cluster."}
	bootstrapPhaseCommand                   = Command{Name: "bootstrap-phase", Help: "Commands to run a single phase of the bootstrap process."}
	executeBashibleSubcommand               = Command{Name: "execute-bashible-bundle", Help: "Prepare Master node and install Kubernetes."}
	createResourcesSubcommand               = Command{Name: "create-resources", Help: "Create resources in Kubernetes cluster."}
	installDeckhouseSubcommand              = Command{Name: "install-deckhouse", Help: "Install deckhouse and wait for its readiness."}
	abortSubcommand                         = Command{Name: "abort", Help: "Delete every node, which was created during bootstrap process."}
	baseInfrastructureSubcommand            = Command{Name: "base-infra", Help: "Create base infrastructure for Cloud Kubernetes cluster."}
	execPostBootstarpSubcommand             = Command{Name: "exec-post-bootstrap", Help: "Test scp upload and ssh run uploaded script."}
	serverCommand                           = Command{Name: "server", Help: "Start dhctl as GRPC server."}
	singleThreadedServerCommand             = Command{Name: "_server", Help: "Start dhctl as GRPC server. Single threaded version."}
	convergeCommand                         = Command{Name: "converge", Help: "Converge kubernetes cluster."}
	autoConvergeCommand                     = Command{Name: "converge-periodical", Help: "Start service for periodical run converge."}
	lockCommand                             = Command{Name: "lock", Help: "Converge cluster lock"}
	lockReleaseSubcommand                   = Command{Name: "release", Help: "Release converge lock fully. It's remove converge lease lock from cluster regardless of owner. Be careful"}
	destroyCommand                          = Command{Name: "destroy", Help: "Destroy Kubernetes cluster."}
	terraformCommand                        = Command{Name: "terraform", Help: "Terraform commands."}
	convergeExporterSubcommand              = Command{Name: "converge-exporter", Help: "Run terraform converge exporter."}
	terraformCheckSubcommand                = Command{Name: "check", Help: "Check differences between state of Kubernetes cluster and Terraform state."}
	configCommand                           = Command{Name: "comfig", Help: "Load, edit and save various dhctl configurations."}
	parseSubcommand                         = Command{Name: "parse", Help: "Parse, validate and output configurations."}
	renderSubcommand                        = Command{Name: "render", Help: "Render transitional configurations."}
	editSubcommand                          = Command{Name: "edit", Help: "Change configuration files in Kubernetes cluster conveniently and safely."}
	parseClusterConfigurationSubcommand     = Command{Name: "cluster-configuration", Help: "Parse configuration and print it."}
	parseCloudDiscoveryDataSubcommand       = Command{Name: "cloud-discovery-data", Help: "Parse cloud discovery data and print it."}
	renderBashibleBundleSubcommand          = Command{Name: "bashible-bundle", Help: "Render bashible bundle."}
	renderKubeadmConfigSubcommand           = Command{Name: "kubeadm-config", Help: "Render kubeadm config."}
	renderMasterBootstrapSubcommand         = Command{Name: "master-bootstrap-scripts", Help: "Render master bootstrap scripts."}
	testCommand                             = Command{Name: "test", Help: "Commands to test the parts of bootstrap and converge process."}
	testSSHConnectionSubcommand             = Command{Name: "ssh-connection", Help: "Test connection via ssh."}
	testKubernetesAPIConnectionSubcommand   = Command{Name: "kubernetes-api-connection", Help: "Test connection to kubernetes api via ssh or directly."}
	testSCPSubcommand                       = Command{Name: "scp", Help: "Test scp file operations."}
	testUploadExecSubcommand                = Command{Name: "upload-exec", Help: "Test scp upload and ssh run uploaded script."}
	testBundleSubcommand                    = Command{Name: "bashible-bundle", Help: "Test upload and execute a bundle."}
	testControlPlaneSubcommand              = Command{Name: "control-plane", Help: "Commands to test control plane nodes."}
	testControlPlaneManagerSubcommand       = Command{Name: "manager", Help: "Test control plane manager is ready."}
	testControlPlaneNodeSubcommand          = Command{Name: "node", Help: "Test control plane node is ready."}
	testDeckhouseSubcommand                 = Command{Name: "deckhouse", Help: "Install and uninstall deckhouse."}
	testDeckhouseCreateDeploymentSubcommand = Command{Name: "create-deployment", Help: "Install deckhouse after terraform is applied successful."}
	testDeckhouseRemoveDeploymentSubcommand = Command{Name: "remove-deployment", Help: "Delete deckhouse deployment."}
	testWaitDeploymentReadySubcommand       = Command{Name: "deployment-ready", Help: "Wait while deployment is ready."}
)

type Command struct {
	Name string
	Help string
}

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

	allowed, _ := checkCommand(serverCommand.Name, allowedCommands)
	if allowed {
		commands.DefineServerCommand(kpApp, kingpin.Command(serverCommand.Name, serverCommand.Help))
	}

	allowed, _ = checkCommand(singleThreadedServerCommand.Name, allowedCommands)
	if allowed {
		commands.DefineSingleThreadedServerCommand(kpApp, kingpin.Command(singleThreadedServerCommand.Name, singleThreadedServerCommand.Help))
	}

	allowed, _ = checkCommand(bootstrapCommand.Name, allowedCommands)
	if allowed {
		bootstrap.DefineBootstrapCommand(kpApp, kingpin.Command(bootstrapCommand.Name, bootstrapCommand.Help))
	}

	allowed, subcommands := checkCommand(bootstrapPhaseCommand.Name, allowedCommands)
	if allowed {
		bootstrapPhaseCmd := kpApp.Command(bootstrapPhaseCommand.Name, bootstrapPhaseCommand.Help)
		{
			if checkSubcommand(executeBashibleSubcommand.Name, subcommands) {
				bootstrap.DefineBootstrapExecuteBashibleCommand(bootstrapPhaseCmd, kingpin.Command(executeBashibleSubcommand.Name, executeBashibleSubcommand.Help))
			}

			if checkSubcommand(installDeckhouseSubcommand.Name, subcommands) {
				bootstrap.DefineBootstrapInstallDeckhouseCommand(bootstrapPhaseCmd, kingpin.Command(installDeckhouseSubcommand.Name, installDeckhouseSubcommand.Help))
			}

			if checkSubcommand(createResourcesSubcommand.Name, subcommands) {
				bootstrap.DefineCreateResourcesCommand(bootstrapPhaseCmd, kingpin.Command(createResourcesSubcommand.Name, createResourcesSubcommand.Help))
			}

			if checkSubcommand(abortSubcommand.Name, subcommands) {
				bootstrap.DefineBootstrapAbortCommand(bootstrapPhaseCmd, kingpin.Command(abortSubcommand.Name, abortSubcommand.Help))
			}

			if checkSubcommand(baseInfrastructureSubcommand.Name, subcommands) {
				bootstrap.DefineBaseInfrastructureCommand(bootstrapPhaseCmd, kingpin.Command(baseInfrastructureSubcommand.Name, baseInfrastructureSubcommand.Help))
			}

			if checkSubcommand(execPostBootstarpSubcommand.Name, subcommands) {
				bootstrap.DefineExecPostBootstrapScript(bootstrapPhaseCmd, kingpin.Command(execPostBootstarpSubcommand.Name, execPostBootstarpSubcommand.Help))
			}
		}
	}

	allowed, _ = checkCommand(convergeCommand.Name, allowedCommands)
	if allowed {
		commands.DefineConvergeCommand(kpApp, kingpin.Command(convergeCommand.Name, convergeCommand.Help))
	}

	allowed, _ = checkCommand(autoConvergeCommand.Name, allowedCommands)
	if allowed {
		commands.DefineAutoConvergeCommand(kpApp, kingpin.Command(autoConvergeCommand.Name, autoConvergeCommand.Help))
	}

	allowed, _ = checkCommand(lockCommand.Name, allowedCommands)
	if allowed {
		lockCmd := kpApp.Command(lockCommand.Name, lockCommand.Help)
		{
			commands.DefineReleaseConvergeLockCommand(lockCmd, kingpin.Command(lockReleaseSubcommand.Name, lockReleaseSubcommand.Help))
		}
	}

	allowed, _ = checkCommand(destroyCommand.Name, allowedCommands)
	if allowed {
		commands.DefineDestroyCommand(kpApp, kingpin.Command(destroyCommand.Name, destroyCommand.Help))
	}

	allowed, subcommands = checkCommand(terraformCommand.Name, allowedCommands)
	if allowed {
		terraformCmd := kpApp.Command(terraformCommand.Name, terraformCommand.Help)
		{
			if checkSubcommand(convergeExporterSubcommand.Name, subcommands) {
				commands.DefineTerraformConvergeExporterCommand(terraformCmd, kingpin.Command(convergeExporterSubcommand.Name, convergeExporterSubcommand.Help))
			}

			if checkSubcommand(terraformCheckSubcommand.Name, subcommands) {
				commands.DefineTerraformCheckCommand(terraformCmd, kingpin.Command(terraformCheckSubcommand.Name, terraformCheckSubcommand.Help))
			}
		}
	}

	allowed, subcommands = checkCommand(configCommand.Name, allowedCommands)
	if allowed {
		configCmd := kpApp.Command(configCommand.Name, configCommand.Help)
		{
			if checkSubcommand(parseSubcommand.Name, subcommands) {
				parseCmd := configCmd.Command(parseSubcommand.Name, parseSubcommand.Help)
				{
					commands.DefineCommandParseClusterConfiguration(kpApp, parseCmd, kingpin.Command(parseClusterConfigurationSubcommand.Name, parseClusterConfigurationSubcommand.Help))
					commands.DefineCommandParseCloudDiscoveryData(kpApp, parseCmd, kingpin.Command(parseCloudDiscoveryDataSubcommand.Name, parseCloudDiscoveryDataSubcommand.Help))
				}
			}

			if checkSubcommand(renderSubcommand.Name, subcommands) {
				renderCmd := configCmd.Command(renderSubcommand.Name, renderSubcommand.Help)
				{
					commands.DefineRenderBashibleBundle(renderCmd, kingpin.Command(renderBashibleBundleSubcommand.Name, renderBashibleBundleSubcommand.Help))
					commands.DefineRenderKubeadmConfig(renderCmd, kingpin.Command(renderKubeadmConfigSubcommand.Name, renderKubeadmConfigSubcommand.Help))
					commands.DefineRenderMasterBootstrap(renderCmd, kingpin.Command(renderMasterBootstrapSubcommand.Name, renderMasterBootstrapSubcommand.Help))
				}
			}

			if checkSubcommand(editSubcommand.Name, subcommands) {
				editCmd := configCmd.Command(editSubcommand.Name, editSubcommand.Help)
				{
					commands.DefineEditCommands(editCmd /* wConnFlags */, true)
				}
			}
		}
	}

	allowed, subcommands = checkCommand(testCommand.Name, allowedCommands)
	if allowed {
		testCmd := kpApp.Command(testCommand.Help, testCommand.Help)
		{
			if checkSubcommand(testSSHConnectionSubcommand.Name, subcommands) {
				commands.DefineTestSSHConnectionCommand(testCmd, kingpin.Command(testSSHConnectionSubcommand.Name, testSSHConnectionSubcommand.Help))
			}

			if checkSubcommand(testKubernetesAPIConnectionSubcommand.Name, subcommands) {
				commands.DefineTestKubernetesAPIConnectionCommand(testCmd, kingpin.Command(testKubernetesAPIConnectionSubcommand.Name, testKubernetesAPIConnectionSubcommand.Help))
			}

			if checkSubcommand(testSCPSubcommand.Name, subcommands) {
				commands.DefineTestSCPCommand(testCmd, kingpin.Command(testSCPSubcommand.Name, testSCPSubcommand.Help))
			}

			if checkSubcommand(testUploadExecSubcommand.Name, subcommands) {
				commands.DefineTestUploadExecCommand(testCmd, kingpin.Command(testUploadExecSubcommand.Name, testUploadExecSubcommand.Help))
			}

			if checkSubcommand(testBundleSubcommand.Name, subcommands) {
				commands.DefineTestBundle(testCmd, kingpin.Command(testBundleSubcommand.Name, testBundleSubcommand.Help))
			}

			if checkSubcommand(testControlPlaneSubcommand.Name, subcommands) {
				controlPlaneCmd := testCmd.Command(testControlPlaneSubcommand.Name, testControlPlaneSubcommand.Help)
				{
					commands.DefineTestControlPlaneManagerReadyCommand(controlPlaneCmd, kingpin.Command(testControlPlaneManagerSubcommand.Name, testControlPlaneManagerSubcommand.Help))
					commands.DefineTestControlPlaneNodeReadyCommand(controlPlaneCmd, kingpin.Command(testControlPlaneNodeSubcommand.Name, testControlPlaneNodeSubcommand.Help))
				}
			}
		}

		if checkSubcommand(testDeckhouseSubcommand.Name, subcommands) {
			deckhouseCmd := testCmd.Command(testDeckhouseSubcommand.Name, testDeckhouseSubcommand.Help)
			{
				commands.DefineDeckhouseCreateDeployment(deckhouseCmd, kingpin.Command(testDeckhouseCreateDeploymentSubcommand.Name, testDeckhouseCreateDeploymentSubcommand.Help))
				commands.DefineDeckhouseRemoveDeployment(deckhouseCmd, kingpin.Command(testDeckhouseRemoveDeploymentSubcommand.Name, testDeckhouseRemoveDeploymentSubcommand.Help))
				commands.DefineWaitDeploymentReadyCommand(deckhouseCmd, kingpin.Command(testWaitDeploymentReadySubcommand.Name, testWaitDeploymentReadySubcommand.Help))
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
