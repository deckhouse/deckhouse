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
	"sync"
	"time"

	terminal "golang.org/x/term"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var (
	allowedCommands []string
	commandList     = []Command{
		{
			Name:       "server",
			Help:       "Start dhctl as GRPC server.",
			DefineFunc: commands.DefineServerCommand,
		},
		{
			Name:       "_server",
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

type Command struct {
	Name       string
	Help       string
	DefineFunc func(cmd *kingpin.CmdClause) *kingpin.CmdClause
	Parrent    string
	cmd        *kingpin.CmdClause
}

func main() {
	_ = os.Mkdir(app.TmpDirName, 0o755)

	initGlobalVars()

	tomb.RegisterOnShutdown("Trace", EnableTrace())
	tomb.RegisterOnShutdown("Restore terminal if needed", restoreTerminal())
	tomb.RegisterOnShutdown("Stop default SSH session", process.DefaultSession.Stop)
	tomb.RegisterOnShutdown("Clear dhctl temporary directory", cache.ClearTemporaryDirs)
	tomb.RegisterOnShutdown("Clear infrastructure data temporary directory", cache.ClearInfrastructureDir)

	go tomb.WaitForProcessInterruption()

	kpApp := kingpin.New(app.AppName, "A tool to create Kubernetes cluster and infrastructure.")
	kpApp.HelpFlag.Short('h')
	app.GlobalFlags(kpApp)

	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	err := registerCommands(kpApp)
	if err != nil {
		panic(err)
	}

	runApplication(kpApp)
}

type initer struct {
	logFileMutex sync.Mutex
	logFile      string
}

func newIniter() *initer {
	return &initer{}
}

func (i *initer) initLogger(c *kingpin.ParseContext) error {
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

	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	i.logFile = logPath

	return nil
}

func (i *initer) getLoggerPath() string {
	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	return i.logFile
}

func runApplication(kpApp *kingpin.Application) {
	init := newIniter()

	kpApp.Action(func(c *kingpin.ParseContext) error {
		if err := init.initLogger(c); err != nil {
			return err
		}

		tomb.RegisterOnShutdown("Cleanup providers from default cache", func() {
			infrastructureprovider.CleanupProvidersFromDefaultCache(log.GetDefaultLogger())
		})

		return nil
	})

	kpApp.Version(app.AppVersion).Author("Flant")

	go func() {
		command, err := kpApp.Parse(os.Args[1:])
		errorCode := 0
		if err != nil {
			log.DebugLn(command)

			msg := err.Error()

			if logFile := init.getLoggerPath(); logFile != "" {
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

func getParrentIndex(commandList []Command, name string) (int, error) {
	for i, cmd := range commandList {
		if name == cmd.Name {
			return i, nil
		}
	}

	return -1, fmt.Errorf("parrent command %s not found in command list", name)
}

func getNestingDepth(cmd Command, commands []Command) (Command, int) {
	depth := 0
	visited := make(map[string]bool)
	topLevel := cmd

	for {
		found := false
		for _, c := range commands {
			if c.Name == cmd.Parrent && !visited[c.Name] {
				visited[c.Name] = true
				cmd = c
				depth++
				topLevel = cmd
				found = true
				break
			}
		}

		if !found || cmd.Parrent == "" {
			break
		}
	}

	return topLevel, depth
}

func initParrent(parrentCmdIndex int, kpApp *kingpin.Application) *kingpin.CmdClause {
	var pcmd *kingpin.CmdClause

	if commandList[parrentCmdIndex].cmd == nil {
		pcmd = kpApp.Command(commandList[parrentCmdIndex].Name, commandList[parrentCmdIndex].Help)
		commandList[parrentCmdIndex].cmd = pcmd
	} else {
		pcmd = commandList[parrentCmdIndex].cmd
	}
	return pcmd
}

func registerCommands(kpApp *kingpin.Application) error {
	for i, command := range commandList {
		firstNode, depth := getNestingDepth(command, commandList)
		if depth == 0 {
			allowed, _ := checkCommand(command.Name, allowedCommands)
			if allowed {
				cmd := kpApp.Command(command.Name, command.Help)
				commandList[i].cmd = cmd

				if command.DefineFunc != nil {
					command.DefineFunc(cmd)
				}
			}
		} else {
			parrentCmdIndex, err := getParrentIndex(commandList, command.Parrent)
			if err != nil {
				return err
			}

			allowed, subcommands := checkCommand(firstNode.Name, allowedCommands)

			if allowed && checkSubcommand(command.Name, subcommands) {
				pcmd := initParrent(parrentCmdIndex, kpApp)

				cmd := pcmd.Command(command.Name, command.Help)
				commandList[i].cmd = cmd

				if command.DefineFunc != nil {
					command.DefineFunc(cmd)
				}
			}
		}
	}

	return nil
}
