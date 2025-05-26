/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"report-updater/internal/web"
)

var config web.ServerConfig

func init() {
	var renewInterval string
	logger := log.New(os.Stdout, "", log.LstdFlags)
	flag.StringVar(&renewInterval, "renewInterval", "6h", "Bdu dictionary renew interval (e.g. \"30m\")")
	flag.Parse()

	duration, err := time.ParseDuration(renewInterval)
	if err != nil {
		logger.Fatalf("couldn't parse renew interval value: %v", err)
	}

	config.Logger = logger
	config.HandlerSettings.DictRenewInterval = duration
}

func main() {
	server, err := web.NewServer(&config)
	if err != nil {
		config.Logger.Fatal(err)
	}

	if err = server.Run(context.Background()); err != nil {
		config.Logger.Fatal(err)
	}
}
