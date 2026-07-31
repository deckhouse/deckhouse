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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"

	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/manager"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	mode, err := parseManagerMode()
	if err != nil {
		log.Fatal("Failed to parse manager mode", log.Err(err))
	}

	builder, err := manager.NewBuilder(mode)
	if err != nil {
		log.Fatal("Failed to create manager builder", log.Err(err))
	}

	manager, err := builder.Build(ctx)
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
		log.Info("Bye")
		break
	}
}

func parseManagerMode() (constants.ControlPlaneType, error) {
	modeFlag := flag.String("mode", string(constants.ControlPlaneTypeNormal), "control plane manager mode: normal or virtual")
	flag.Parse()

	mode := *modeFlag
	if flag.NArg() > 0 {
		if flag.NArg() != 1 {
			return "", fmt.Errorf("expected at most one positional mode argument, got %d", flag.NArg())
		}
		mode = flag.Arg(0)
	}

	switch constants.ControlPlaneType(mode) {
	case constants.ControlPlaneTypeNormal:
		return constants.ControlPlaneTypeNormal, nil
	case constants.ControlPlaneTypeVirtual:
		return constants.ControlPlaneTypeVirtual, nil
	default:
		return "", fmt.Errorf("unsupported mode %q", mode)
	}
}
