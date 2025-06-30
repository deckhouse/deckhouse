/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"d8_shutdown_inhibitor/pkg/app"
)

func main() {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	// Start application.
	app := app.NewApp(app.AppConfig{
		PodLabel:              app.InhibitNodeShutdownLabel,
		InhibitDelayMax:       app.InhibitDelayMaxSec,
		PodsCheckingInterval:  app.PodsCheckingInterval,
		WallBroadcastInterval: app.WallBroadcastInterval,
		NodeName:              nodeName,
	})

	err = app.Start()
	if err != nil {
		fmt.Printf("START Error: %s\n", err.Error())
		os.Exit(1)
	}

	// Wait for signal to stop application.
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-interruptCh:
		fmt.Printf("Grace shutdown by '%s' signal\n", sig.String())
		app.Stop()
		<-app.Done()
	case <-app.Done():
		fmt.Printf("Application stopped\n")
	}

	err = app.Err()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		os.Exit(1)
	}
}
