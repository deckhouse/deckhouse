package main

import (
	"fmt"
	"os"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-cluster/cmd/deckhouse-cluster/commands"
	"flant/deckhouse-cluster/pkg/app"
)

func main() {
	// kubectl compatibility
	sh_app.KubeConfig = os.Getenv("KUBECONFIG")

	kpApp := kingpin.New(app.AppName, "")

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.AppVersion)
		return nil
	})

	// bootstrap
	bootstrapCmd := commands.GetBootstrapCommand(kpApp)
	app.DefineConfigFlags(bootstrapCmd)
	app.DefineSshFlags(bootstrapCmd)

	// konverge
	konvergeCmd := commands.GetKonvergeCommand(kpApp)
	app.DefineKonvergeFlags(konvergeCmd)
	app.DefineSshFlags(konvergeCmd)
	//sh_app.DefineKubeClientFlags(konvergeCmd)

	helperCmd := kpApp.Command("helper", "Plumbing commands.")

	// plumbing commands:
	{
		runBaseTerraformCmd := commands.GetRunBaseTerraformCommand(helperCmd)
		app.DefineConfigFlags(runBaseTerraformCmd)

		runMasterTerraformCmd := commands.GetRunMasterTerraformCommand(helperCmd)
		app.DefineConfigFlags(runMasterTerraformCmd)

		app.DefineCommandParseClusterConfiguration(kpApp, helperCmd)

		sshCmd := commands.GetTestSshConnectionCommand(helperCmd)
		app.DefineSshFlags(sshCmd)

		kubeCmd := commands.GetTestKubernetesAPIConnectionCommand(helperCmd)
		app.DefineSshFlags(kubeCmd)
		sh_app.DefineKubeClientFlags(kubeCmd)

		helperCmd.Command("generate-master-nodes-manifests", "Not implemented yet.")
		helperCmd.Command("generate-bashible-bundle", "Not implemented yet.")
	}

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
