package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-cluster/pkg/config"
	"flant/deckhouse-cluster/pkg/deckhouse"
	"flant/deckhouse-cluster/pkg/kube"
	"flant/deckhouse-cluster/pkg/terraform"
)

func GetKonvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	return kpApp.Command("konverge", "Converge kubernetes cluster.").
		Action(func(c *kingpin.ParseContext) error {
			// Open connection to kubernetes API
			kubeCl := kube.NewKubernetesClient()
			// auto init
			err := kubeCl.Init("")
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			// defer stop ssh-agent, proxy and a tunnel
			defer kubeCl.Stop()

			err = deckhouse.RunKonverge(
				kubeCl,
				terraform.NewPipeline("tf_base", new(config.MetaConfig), terraform.GetBasePipelineResult),
			)

			if err != nil {
				return fmt.Errorf("konverge error: %v", err)
			}
			return nil
		})
}
