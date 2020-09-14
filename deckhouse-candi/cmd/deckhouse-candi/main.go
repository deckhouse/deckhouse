package main

import (
	"fmt"
	"os"
	"runtime/trace"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/cmd/deckhouse-candi/commands"
	"flant/deckhouse-candi/cmd/deckhouse-candi/commands/bootstrap"
	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/process"
	"flant/deckhouse-candi/pkg/util/signal"
)

func main() {
	defer EnableTrace()()

	// kill all started subprocesses on return from main or on signal
	defer process.DefaultSession.Stop()
	go func() {
		signal.WaitForProcessInterruption(func() {
			process.DefaultSession.Stop()
		})
	}()

	_ = os.Mkdir(app.TmpDirName, 0755)

	kpApp := kingpin.New(app.AppName, "A tool to create Kubernetes cluster and infrastructure.")
	kpApp.HelpFlag.Short('h')
	app.GlobalFlags(kpApp)
	// kpApp.UsageTemplate(kingpin.CompactUsageTemplate)

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	// bootstrap
	bootstrap.DefineBootstrapCommand(kpApp)
	bootstrapPhaseCmd := kpApp.Command("bootstrap-phase", "Commands to run a single phase of the bootstrap process.")
	{
		bootstrap.DefineBootstrapExecuteBashibleCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapInstallDeckhouseCommand(bootstrapPhaseCmd)
		bootstrap.DefineCreateResourcesCommand(bootstrapPhaseCmd)
		bootstrap.DefineBootstrapAbortCommand(bootstrapPhaseCmd)
	}

	// converge
	commands.DefineConvergeCommand(kpApp)

	// destroy
	commands.DefineDestroyCommand(kpApp)

	// plumbing commands:
	terraformCmd := kpApp.Command("terraform", "Terraform commands.")
	{
		commands.DefineTerraformConvergeExporterCommand(terraformCmd)
		commands.DefineTerraformCheckCommand(terraformCmd)
	}

	renderCmd := kpApp.Command("render", "Parse, validate and render bundles and configs.")
	{
		commands.DefineCommandParseClusterConfiguration(kpApp, renderCmd)
		commands.DefineCommandParseCloudDiscoveryData(kpApp, renderCmd)
		commands.DefineRenderBashibleBundle(renderCmd)
		commands.DefineRenderKubeadmConfig(renderCmd)
	}

	testCmd := kpApp.Command("test", "Commands to test the parts of bootstrap process.")
	{
		commands.DefineTestSshConnectionCommand(testCmd)
		commands.DefineTestKubernetesAPIConnectionCommand(testCmd)
		commands.DefineTestScpCommand(testCmd)
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
	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}

func EnableTrace() func() {
	fName := os.Getenv("CANDI_TRACE")
	if fName == "" || fName == "0" || fName == "no" {
		return func() {}
	}
	if fName == "1" || fName == "yes" {
		fName = "trace.out"
	}

	fns := make([]func(), 0)

	f, err := os.Create(fName)
	if err != nil {
		fmt.Printf("failed to create trace output file '%s': %v", fName, err)
		os.Exit(1)
	}
	fns = append([]func(){
		func() {
			if err := f.Close(); err != nil {
				fmt.Printf("failed to close trace file '%s': %v", fName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	if err := trace.Start(f); err != nil {
		fmt.Printf("failed to start trace to '%s': %v", fName, err)
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
