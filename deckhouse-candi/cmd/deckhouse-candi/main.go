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

	helperCmd := kpApp.Command("helper", "Plumbing commands.")

	// plumbing commands:
	{
		commands.DefineRunBaseTerraformCommand(helperCmd)

		commands.DefineRunMasterTerraformCommand(helperCmd)

		commands.DefineRunDestroyAllTerraformCommand(helperCmd)

		app.DefineCommandParseClusterConfiguration(kpApp, helperCmd)

		commands.DefineTestSshConnectionCommand(helperCmd)

		commands.DefineTestKubernetesAPIConnectionCommand(helperCmd)

		commands.DefineTestScpCommand(helperCmd)

		commands.DefineTestUploadExecCommand(helperCmd)

		commands.DefineTestBundle(helperCmd)

		helperCmd.Command("generate-master-nodes-manifests", "Not implemented yet.")
		helperCmd.Command("generate-bashible-bundle", "Not implemented yet.")
	}

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
