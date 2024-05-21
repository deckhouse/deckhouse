/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package manager

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	common_config "system-registry-manager/internal/manager/common"
	pkg_logs "system-registry-manager/pkg/logs"
)

func StartOsSignalHandler(rootCtx context.Context, rCfg *common_config.RuntimeConfig, stopFuncs ...func()) {
	log := pkg_logs.GetLoggerFromContext(rootCtx)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		log.Infof("Received signal: %v", sig)
		rCfg.StopManager()
	case <-rootCtx.Done():
		log.Error("Root context cancelled")
	}
	log.Info("Os signal handler shutdown")

	for _, f := range stopFuncs {
		f()
	}
}
