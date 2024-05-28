/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package worker

import (
	"context"
	log "github.com/sirupsen/logrus"
	"net/http"
	common "system-registry-manager/internal/common"
	pkg_api "system-registry-manager/pkg/api"
	pkg_logs "system-registry-manager/pkg/logs"
	"time"
)

const (
	processName     = "worker"
	shutdownTimeout = 5 * time.Second
)

type Worker struct {
	WorkerData
	WorkerServer
}

type WorkerData struct {
	commonCfg        *common.RuntimeConfig
	rootCtx          context.Context
	log              *log.Entry
	singleRequestCfg *pkg_api.SingleRequestConfig
}
type WorkerServer struct {
	server *http.Server
}

func New(rootCtx context.Context, rCfg *common.RuntimeConfig) *Worker {
	rootCtx = pkg_logs.SetLoggerToContext(rootCtx, processName)
	log := pkg_logs.GetLoggerFromContext(rootCtx)

	worker := &Worker{
		WorkerData{
			commonCfg:        rCfg,
			rootCtx:          rootCtx,
			log:              log,
			singleRequestCfg: pkg_api.CreateSingleRequestConfig(),
		},
		WorkerServer{
			server: nil,
		},
	}

	worker.server = createServer(&worker.WorkerData)
	return worker
}

func (m *Worker) Start() {
	m.log.Info("Worker starting...")
	if err := m.server.ListenAndServe(); err != nil {
		defer m.commonCfg.StopManager()
		if err != http.ErrServerClosed {
			m.log.Errorf("error starting server: %v", err)
		} else {
			m.log.Errorf("error, server stopped: %v", err)
		}
	}
}

func (m *Worker) Stop() {
	m.log.Info("Worker shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := m.server.Shutdown(ctx); err != nil {
		m.log.Errorf("error shutting down server: %v", err)
	}
	m.log.Info("Worker shutdown")
}
