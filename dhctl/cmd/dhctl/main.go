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
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"runtime/trace"
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

	commands.DefineMirrorCommand(kpApp)
	commands.DefineMirrorModulesCommand(kpApp)

	commands.DefineServerCommand(kpApp)
	commands.DefineSingleThreadedServerCommand(kpApp)

	bootstrap.DefineBootstrapCommand(kpApp)
	bootstrapPhaseCmd := kpApp.Command("bootstrap-phase", "Commands to run a single phase of the bootstrap process.")
	{
		bootstrap.DefineBootstrapExecuteBashibleCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapInstallDeckhouseCommand(bootstrapPhaseCmd)
		bootstrap.DefineCreateResourcesCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapAbortCommand(bootstrapPhaseCmd)
		bootstrap.DefineBaseInfrastructureCommand(bootstrapPhaseCmd)
		bootstrap.DefineExecPostBootstrapScript(bootstrapPhaseCmd)
	}

	commands.DefineConvergeCommand(kpApp)
	commands.DefineAutoConvergeCommand(kpApp)

	lockCmd := kpApp.Command("lock", "Converge cluster lock")
	{
		commands.DefineReleaseConvergeLockCommand(lockCmd)
	}

	commands.DefineDestroyCommand(kpApp)

	terraformCmd := kpApp.Command("terraform", "Terraform commands.")
	{
		commands.DefineTerraformConvergeExporterCommand(terraformCmd)
		commands.DefineTerraformCheckCommand(terraformCmd)
	}

	configCmd := kpApp.Command("config", "Load, edit and save various dhctl configurations.")
	{
		parseCmd := configCmd.Command("parse", "Parse, validate and output configurations.")
		{
			commands.DefineCommandParseClusterConfiguration(kpApp, parseCmd)
			commands.DefineCommandParseCloudDiscoveryData(kpApp, parseCmd)
		}

		renderCmd := configCmd.Command("render", "Render transitional configurations.")
		{
			commands.DefineRenderBashibleBundle(renderCmd)
			commands.DefineRenderKubeadmConfig(renderCmd)
			commands.DefineRenderMasterBootstrap(renderCmd)
		}

		editCmd := configCmd.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
		{
			commands.DefineEditCommands(editCmd /* wConnFlags */, true)
		}
	}

	testCmd := kpApp.Command("test", "Commands to test the parts of bootstrap and converge process.")
	{
		commands.DefineTestSSHConnectionCommand(testCmd)
		commands.DefineTestKubernetesAPIConnectionCommand(testCmd)
		commands.DefineTestSCPCommand(testCmd)
		commands.DefineTestUploadExecCommand(testCmd)
		commands.DefineTestBundle(testCmd)

		controlPlaneCmd := testCmd.Command("control-plane", "Commands to test control plane nodes.")
		{
			commands.DefineTestControlPlaneManagerReadyCommand(controlPlaneCmd)
			commands.DefineTestControlPlaneNodeReadyCommand(controlPlaneCmd)
		}
	}

	deckhouseCmd := testCmd.Command("deckhouse", "Install and uninstall deckhouse.")
	{
		commands.DefineDeckhouseCreateDeployment(deckhouseCmd)
		commands.DefineDeckhouseRemoveDeployment(deckhouseCmd)
		commands.DefineWaitDeploymentReadyCommand(deckhouseCmd)
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
	traceFile := os.Getenv("DHCTL_TRACE")
	profFile := traceFile + ".prof"
	if traceFile == "" || traceFile == "0" || traceFile == "no" {
		return func() {}
	}
	if traceFile == "1" || traceFile == "yes" {
		traceFile = "trace.out"
		profFile = traceFile + ".prof"
	}

	fns := make([]func(), 0)

	traceF, err := os.Create(traceFile)
	if err != nil {
		log.InfoF("failed to create trace output file '%s': %v", traceFile, err)
		os.Exit(1)
	}

	profF, err := os.Create(profFile)
	if err != nil {
		log.InfoF("failed to create trace output file '%s': %v", profFile, err)
		os.Exit(1)
	}

	fns = append([]func(){
		func() {
			if err := traceF.Close(); err != nil {
				log.InfoF("failed to close trace file '%s': %v", traceFile, err)
				os.Exit(1)
			}
		},
	}, fns...)

	if err := trace.Start(traceF); err != nil {
		log.InfoF("failed to start trace to '%s': %v", traceFile, err)
		os.Exit(1)
	}
	fns = append([]func(){
		trace.Stop,
	}, fns...)

	mux := http.NewServeMux()
	mux.HandleFunc("/profile", pprof.Profile)

	server := &http.Server{Addr: "127.0.0.1:17777", Handler: mux, ReadHeaderTimeout: 30 * time.Second}

	start := time.Now()

	go server.ListenAndServe()

	fns = append([]func(){
		func() {
			defer profF.Close()

			end := time.Now()

			dur := end.Sub(start)
			seconds := int64(dur.Seconds())
			log.InfoF("Profiling took %v seconds", seconds)
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:17777/profile?seconds=%d", seconds))
			if err != nil {
				log.ErrorLn(err)
				return
			}

			defer resp.Body.Close()

			if _, err := io.Copy(profF, resp.Body); err != nil {
				log.ErrorLn(err)
				return
			}

			ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				log.ErrorLn(err)
				return
			}
		},
	})

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
	// get current path
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// set path to ssh and terraform binaries
	if err := os.Setenv("PATH", fmt.Sprintf("%s/bin:%s", pwd, os.Getenv("PATH"))); err != nil {
		panic(err)
	}

	// set relative path to config and template files
	config.InitGlobalVars(pwd)
	commands.InitGlobalVars(pwd)
	app.InitGlobalVars(pwd)
	terraform.InitGlobalVars(pwd)
	manifests.InitGlobalVars(pwd)
	template.InitGlobalVars(pwd)
}
