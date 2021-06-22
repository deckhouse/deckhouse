package main

import (
	"fmt"
	"os"
	"runtime/trace"

	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/process"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func main() {
	_ = os.Mkdir(app.TmpDirName, 0o755)

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

	bootstrap.DefineBootstrapCommand(kpApp)
	bootstrapPhaseCmd := kpApp.Command("bootstrap-phase", "Commands to run a single phase of the bootstrap process.")
	{
		bootstrap.DefineBootstrapExecuteBashibleCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapInstallDeckhouseCommand(bootstrapPhaseCmd)
		bootstrap.DefineCreateResourcesCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapAbortCommand(bootstrapPhaseCmd)
		bootstrap.DefineBaseInfrastructureCommand(bootstrapPhaseCmd)
	}

	commands.DefineConvergeCommand(kpApp)
	commands.DefineAutoConvergeCommand(kpApp)

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
		}

		editCmd := configCmd.Command("edit", "Change configuration files in Kubernetes cluster conveniently and safely.")
		{
			commands.DefineEditClusterConfigurationCommand(editCmd)
			commands.DefineEditProviderClusterConfigurationCommand(editCmd)
			commands.DefineEditStaticClusterConfigurationCommand(editCmd)
		}
	}

	testCmd := kpApp.Command("test", "Commands to test the parts of bootstrap process.")
	{
		commands.DefineTestSSHConnectionCommand(testCmd)
		commands.DefineTestKubernetesAPIConnectionCommand(testCmd)
		commands.DefineTestSCPCommand(testCmd)
		commands.DefineTestUploadExecCommand(testCmd)
		commands.DefineTestBundle(testCmd)
	}

	deckhouseCmd := testCmd.Command("deckhouse", "Install and uninstall deckhouse.")
	{
		commands.DefineDeckhouseCreateDeployment(deckhouseCmd)
		commands.DefineDeckhouseRemoveDeployment(deckhouseCmd)
		commands.DefineWaitDeploymentReadyCommand(deckhouseCmd)
	}

	kpApp.Action(func(c *kingpin.ParseContext) error {
		log.InitLogger(app.LoggerType)
		return nil
	})

	kpApp.Version("v0.1.0").Author("Flant")

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
	fName := os.Getenv("DHCTL_TRACE")
	if fName == "" || fName == "0" || fName == "no" {
		return func() {}
	}
	if fName == "1" || fName == "yes" {
		fName = "trace.out"
	}

	fns := make([]func(), 0)

	f, err := os.Create(fName)
	if err != nil {
		log.InfoF("failed to create trace output file '%s': %v", fName, err)
		os.Exit(1)
	}
	fns = append([]func(){
		func() {
			if err := f.Close(); err != nil {
				log.InfoF("failed to close trace file '%s': %v", fName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	if err := trace.Start(f); err != nil {
		log.InfoF("failed to start trace to '%s': %v", fName, err)
		os.Exit(1)
	}
	fns = append([]func(){
		trace.Stop,
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
