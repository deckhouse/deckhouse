/*
Copyright 2026 Flant JSC

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
	"context"
	"control-plane-manager/internal"
	"control-plane-manager/internal/constants"
	"os"
	"os/signal"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	manager, err := internal.NewManager(ctx, false) // TODO pprof flag
	if err != nil {
		log.Fatal("Failed to create a manager", log.Err(err))
	}

	if err = manager.Start(ctx); err != nil {
		log.Fatal("Failed to start the manager", log.Err(err))
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for range sigs {
		log.Info("Shutdown signal received")
		cancel()
		log.Info("Bye from %s", constants.CpcControllerName)
		break
	}
}
