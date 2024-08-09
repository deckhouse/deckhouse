/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"context"
	"sync"
	"system-registry-manager/internal/executor"
	"system-registry-manager/internal/master"
	pkg_logs "system-registry-manager/pkg/logs"
)

const mainProcessName = "main"

func StartManager() {
	rootCtx, rootCtxCancel := context.WithCancel(context.Background())
	defer rootCtxCancel()

	// Set logger to context
	rootCtx = pkg_logs.SetLoggerToContext(rootCtx, mainProcessName)
	log := pkg_logs.GetLoggerFromContext(rootCtx)

	executor := executor.New(rootCtx, rootCtxCancel)
	master := master.New(rootCtx, rootCtxCancel)

	var wg sync.WaitGroup
	wg.Add(3) // Changed the value to 2 since we have only two executor goroutines

	// Goroutine for handling signals
	go func() {
		defer wg.Done()
		log.Info("Starting os signal handler...")
		StartOsSignalHandler(rootCtx, rootCtxCancel, executor.Stop)
	}()

	// Start executor goroutine
	go func() {
		defer wg.Done()
		log.Info("Starting executor...")
		executor.Start()
	}()

	// Start master goroutine
	go func() {
		defer wg.Done()
		log.Info("Starting master...")
		master.Start()
	}()

	wg.Wait()
	log.Info("Shutting down...")
}
