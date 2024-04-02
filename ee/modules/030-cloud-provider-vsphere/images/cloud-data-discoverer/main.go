/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/sirupsen/logrus"

	cloud_data "github.com/deckhouse/deckhouse/go_lib/cloud-data"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/app"
)

func main() {
	kpApp := kingpin.New("vsphere cloud cloud data discoverer", "A tool for discovery data from cloud provider")
	kpApp.HelpFlag.Short('h')

	app.InitFlags(kpApp)

	kpApp.Action(func(context *kingpin.ParseContext) error {
		logger := app.InitLogger()
		client := app.InitClient(logger)
		dynamicClient := app.InitDynamicClient(logger)
		discoverer := NewDiscoverer(logger)

		r := cloud_data.NewReconciler(discoverer, app.ListenAddress, app.DiscoveryPeriod, logger, client, dynamicClient)
		r.Start()

		return nil
	})

	_, err := kpApp.Parse(os.Args[1:])
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
