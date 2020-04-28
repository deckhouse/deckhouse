package main

import (
	"fmt"
	"github.com/flant/logboek"
	"os"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/cmd/deckhouse-candi/commands"
	"flant/deckhouse-candi/pkg/app"
)

func main() {
	logboek.Init()
	logboek.SetLevel(logboek.Info)

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
		commands.DefineDeckhouseCreateDeployment(deckhouseCmd)
		commands.DefineDeckhouseRemoveDeployment(deckhouseCmd)
	}

	kpApp.Version("v0.1.0").Author("Flant")
	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
