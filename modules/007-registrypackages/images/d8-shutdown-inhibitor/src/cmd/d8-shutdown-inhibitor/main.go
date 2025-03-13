/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"graceful_shutdown/pkg/app"
	"graceful_shutdown/pkg/debug"
	"graceful_shutdown/pkg/inputdev"
)

// Settings
const (
	InhibitDelayMaxSec       = 3 * 24 * time.Hour // 3 days
	InhibitNodeShutdownLabel = "pod.deckhouse.io/inhibit-node-shutdown"
	WallBroadcastPeriod      = 30 * time.Second
)

func main() {
	if RunDebugCommand(os.Args) {
		return
	}

	RunInhibitorApp()
}

func RunInhibitorApp() {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	// Start application.
	app := app.NewApp(app.AppConfig{
		InhibitDelayMax:     InhibitDelayMaxSec,
		WallBroadcastPeriod: WallBroadcastPeriod,
		PodLabel:            InhibitNodeShutdownLabel,
		NodeName:            nodeName,
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

func RunDebugCommand(args []string) bool {
	if len(args) < 1 {
		return false
	}

	switch args[1] {
	case "list-pods":
		debug.ListPods(InhibitNodeShutdownLabel)
	case "list-input-devices":
		debug.ListInputDevices()
	case "watch-for-key":
		debug.WatchForKey(inputdev.KEY_Q, inputdev.KEY_E, inputdev.KEY_W, inputdev.KEY_ENTER)
	default:
		return false
	}

	return true
}
