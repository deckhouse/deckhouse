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
	"graceful_shutdown/pkg/inputdev"
)

func main() {

	args1 := ""
	if len(os.Args) > 1 {
		args1 = os.Args[1]
	}

	if args1 == "list-input-devices" {
		devs, err := inputdev.ListInputDevicesWithAnyButton(inputdev.KEY_POWER, inputdev.KEY_POWER2)
		if err != nil {
			fmt.Printf("list power key devices: %w", err)
			os.Exit(1)
		}

		for _, dev := range devs {
			fmt.Printf("Device: %s, %s\n", dev.Name, dev.DevPath)
		}
		os.Exit(0)
	}

	if args1 == "watch-for-key" {
		buttons := []inputdev.Button{
			inputdev.KEY_Q, inputdev.KEY_E, inputdev.KEY_W, inputdev.KEY_ENTER,
		}
		devs, err := inputdev.ListInputDevicesWithAnyButton(buttons...)
		if err != nil {
			fmt.Printf("list devices with Q W E Enter: %w", err)
			os.Exit(1)
		}

		for _, dev := range devs {
			fmt.Printf("Device: %s, %s\n", dev.Name, dev.DevPath)
		}

		watcher := inputdev.NewWatcher(devs, buttons...)
		watcher.Start()
		fmt.Printf("watch for button press\n")
		<-watcher.Pressed()
		fmt.Printf("button was pressed\n")
		os.Exit(0)
	}

	checkOnlyPods := false
	if args1 == "checkpods" {
		checkOnlyPods = true
	}

	// Application settings
	maxDelay := 30 * 60 * time.Second // 30 minutes
	podLabel := "pod.deckhouse.io/inhibit-node-shutdown"

	// Start application.
	app := app.NewApp(maxDelay, podLabel)

	if checkOnlyPods {
		app.CheckPods()
		os.Exit(0)
	}

	err := app.Start()
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
