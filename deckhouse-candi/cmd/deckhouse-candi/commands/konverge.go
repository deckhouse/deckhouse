package commands

import (
	"flant/deckhouse-candi/pkg/app"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/kube"
)

func DefineKonvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("konverge", "Converge kubernetes cluster.")
	app.DefineKonvergeFlags(cmd)
	app.DefineSshFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		// Open connection to kubernetes API
		kubeCl := kube.NewKubernetesClient()
		// auto init
		err := kubeCl.Init("")
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		// err = deckhouse.RunKonverge(
		//	kubeCl,
		//	terraform.NewPipeline("tf_base", "", new(config.MetaConfig), terraform.GetBasePipelineResult),
		//)

		if err != nil {
			return fmt.Errorf("konverge error: %v", err)
		}
		return nil
	})
	return cmd
}
