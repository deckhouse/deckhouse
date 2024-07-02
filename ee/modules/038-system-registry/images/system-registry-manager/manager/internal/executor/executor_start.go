/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package executor

import (
	"context"
	"github.com/sirupsen/logrus"
	"net/http"
	common "system-registry-manager/internal/common"
	executor_client "system-registry-manager/pkg/executor/client"
	pkg_logs "system-registry-manager/pkg/logs"
	"time"
)

const (
	processName     = "executor"
	shutdownTimeout = 5 * time.Second
)

type Executor struct {
	ExecutorData
	ExecutorServer
}

type ExecutorData struct {
	commonCfg        *common.RuntimeConfig
	rootCtx          context.Context
	log              *logrus.Entry
	singleRequestCfg *executor_client.SingleRequestConfig
}
type ExecutorServer struct {
	server *http.Server
}

func New(rootCtx context.Context, rCfg *common.RuntimeConfig) *Executor {
	rootCtx = pkg_logs.SetLoggerToContext(rootCtx, processName)
	log := pkg_logs.GetLoggerFromContext(rootCtx)

	executor := &Executor{
		ExecutorData{
			commonCfg:        rCfg,
			rootCtx:          rootCtx,
			log:              log,
			singleRequestCfg: executor_client.CreateSingleRequestConfig(),
		},
		ExecutorServer{
			server: nil,
		},
	}

	executor.server = createServer(&executor.ExecutorData)
	return executor
}

func (m *Executor) Start() {
	m.log.Info("Executor starting...")
	if err := m.server.ListenAndServe(); err != nil {
		defer m.commonCfg.StopManager()
		if err != http.ErrServerClosed {
			m.log.Errorf("error starting server: %v", err)
		} else {
			m.log.Errorf("error, server stopped: %v", err)
		}
	}
}

func (m *Executor) Stop() {
	m.log.Info("Executor shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := m.server.Shutdown(ctx); err != nil {
		m.log.Errorf("error shutting down server: %v", err)
	}
	m.log.Info("Executor shutdown")
}
