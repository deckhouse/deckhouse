/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/deckhouse/deckhouse/pkg/log"

	cloud_data "github.com/deckhouse/deckhouse/go_lib/cloud-data"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/app"

	vcd "github.com/deckhouse/deckhouse/go_lib/cloud-data/discovery/vcd"
)

func main() {
	kpApp := kingpin.New("vcd cloud data discoverer", "A tool for discovery data from cloud provider")
	kpApp.HelpFlag.Short('h')

	app.InitFlags(kpApp)

	kpApp.Action(func(context *kingpin.ParseContext) error {
		logger := app.InitLogger()
		client := app.InitClient(logger)
		dynamicClient := app.InitDynamicClient(logger)
		config, err := vcd.ParseEnvToConfig()
		if err != nil {
			return fmt.Errorf("error creating discoverer config: %w", err)
		}
		discoverer := vcd.NewDiscoverer(logger, config)

		r := cloud_data.NewReconciler(discoverer, app.ListenAddress, app.DiscoveryPeriod, logger, client, dynamicClient)
		r.Start()

		return nil
	})

	_, err := kpApp.Parse(os.Args[1:])
	if err != nil {
		log.Error("failed to parse command-line arguments", err)
		os.Exit(1)
	}
}
