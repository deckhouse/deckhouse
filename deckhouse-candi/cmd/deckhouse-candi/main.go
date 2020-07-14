package main

import (
	"fmt"
	"os"
	"runtime/trace"

	"github.com/flant/logboek"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/cmd/deckhouse-candi/commands"
	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/process"
	"flant/deckhouse-candi/pkg/util/signal"
)

func main() {
	defer EnableTrace()()

	err := logboek.Init()
	if err != nil {
		panic(fmt.Errorf("can't start logging system: %w", err))
	}
	logboek.SetLevel(logboek.Info)

	// kill all started subprocesses on return from main or on signal
	defer process.DefaultSession.Stop()
	go func() {
		signal.WaitForProcessInterruption(func() {
			process.DefaultSession.Stop()
		})
	}()

	// kubectl compatibility
	sh_app.KubeConfig = os.Getenv("KUBECONFIG")

	kpApp := kingpin.New(app.AppName, "")

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	// bootstrap
	commands.DefineBootstrapCommand(kpApp)

	// konverge
	commands.DefineKonvergeCommand(kpApp)

	// plumbing commands:
	terraformCmd := kpApp.Command("terraform", "Terraform commands.")
	{
		commands.DefineRunBaseTerraformCommand(terraformCmd)
		commands.DefineRunMasterTerraformCommand(terraformCmd)
		commands.DefineRunDestroyAllTerraformCommand(terraformCmd)
	}

	renderCmd := kpApp.Command("render", "Parse, validate and render bundles.")
	{
		app.DefineCommandParseClusterConfiguration(kpApp, renderCmd)
		commands.DefineRenderBashibleBundle(renderCmd)
		commands.DefineRenderKubeadmConfig(renderCmd)
	}

	testCmd := kpApp.Command("test", "Commands to test the parts of bootstrap process.")
	{
		commands.DefineTestSshConnectionCommand(testCmd)
		commands.DefineTestKubernetesAPIConnectionCommand(testCmd)
		commands.DefineWaitDeploymentReadyCommand(testCmd)
		commands.DefineTestScpCommand(testCmd)
		commands.DefineTestUploadExecCommand(testCmd)
		commands.DefineTestBundle(testCmd)
	}

	deckhouseCmd := kpApp.Command("deckhouse", "Install and uninstall deckhouse.")
	{
		commands.DefineDeckhouseInstall(deckhouseCmd)
		commands.DefineDeckhouseCreateDeployment(deckhouseCmd)
		commands.DefineDeckhouseRemoveDeployment(deckhouseCmd)
	}

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
