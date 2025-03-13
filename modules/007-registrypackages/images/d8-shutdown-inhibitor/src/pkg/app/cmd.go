package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func Run() {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	// Start application.
	app := NewApp(AppConfig{
		PodLabel:              InhibitNodeShutdownLabel,
		InhibitDelayMax:       InhibitDelayMaxSec,
		PodsCheckingInterval:  PodsCheckingInterval,
		WallBroadcastInterval: WallBroadcastInterval,
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
