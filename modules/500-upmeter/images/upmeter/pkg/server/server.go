/*
Copyright 2021 Flant JSC

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

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"time"

	kube "github.com/flant/kube-client/client"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/downtime"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/registry"
	"d8.io/upmeter/pkg/server/api"
	"d8.io/upmeter/pkg/server/remotewrite"
)

// server initializes all dependencies:
// - kubernetes client
// - crd monitor
// - database connection
// - metrics storage
// If everything is ok, it starts http server.

type Server struct {
	config     *Config
	kubeConfig *kubernetes.Config

	logger *log.Logger

	server                *http.Server
	downtimeMonitor       *downtime.Monitor
	remoteWriteController *remotewrite.Controller
}

type Config struct {
	ListenHost string
	ListenPort string
	UserAgent  string

	DatabasePath          string
	DatabaseRetentionDays int

	OriginsCount int

	DisabledProbes []string
	DynamicProbes  *DynamicProbesConfig
}

type DynamicProbesConfig struct {
	IngressControllers []string
	NodeGroups         []string
}

func NewConfig() *Config {
	return &Config{
		DynamicProbes: &DynamicProbesConfig{},
	}
}

func New(config *Config, kubeConfig *kubernetes.Config, logger *log.Logger) *Server {
	return &Server{
		config:     config,
		kubeConfig: kubeConfig,
		logger:     logger,
	}
}

func (s *Server) Start(ctx context.Context) error {
	var err error

	kubeClient, err := kubernetes.InitKubeClient(s.kubeConfig)
	if err != nil {
		return fmt.Errorf("init kubernetes client: %v", err)
	}

	// Database connection with pool
	dbctx, err := db.Connect(s.config.DatabasePath, dbcontext.DefaultConnectionOptions())
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	// Downtime CR monitor
	s.downtimeMonitor, err = initDowntimeMonitor(ctx, kubeClient, s.logger)
	if err != nil {
		return fmt.Errorf("cannot start downtimes.deckhouse.io monitor: %v", err)
	}

	// Metrics controller
	s.remoteWriteController, err = initRemoteWriteController(ctx, dbctx, kubeClient, s.config.OriginsCount, s.logger, s.config.UserAgent)
	if err != nil {
		s.logger.Debugf("starting controller... did't happen: %v", err)
		return fmt.Errorf("cannot start remote_write controller: %v", err)
	}

	go cleanOld30sEpisodes(ctx, dbctx)
	go cleanOld5mEpisodes(ctx, dbctx, s.config.DatabaseRetentionDays)

	// Probe lister that can only list groups and probes
	probeLister := newProbeLister(s.config.DisabledProbes, s.config.DynamicProbes)

	// Start http server. It blocks, that's why it is the last here.
	s.logger.Debugf("starting HTTP server")
	listenAddr := s.config.ListenHost + ":" + s.config.ListenPort
	s.server = initHttpServer(dbctx, s.downtimeMonitor, s.remoteWriteController, probeLister, listenAddr)

	err = s.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

func (s *Server) Stop() error {
	err := s.server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	s.remoteWriteController.Stop()
	s.downtimeMonitor.Stop()

	return nil
}

func cleanOld30sEpisodes(ctx context.Context, dbCtx *dbcontext.DbContext) {
	dayBack := -24 * time.Hour
	period := 30 * time.Second

	conn := dbCtx.Start()
	defer conn.Stop()

	storage := dao.NewEpisodeDao30s(conn)

	ticker := time.NewTicker(period)

	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Truncate(period).Add(dayBack)
			err := storage.DeleteUpTo(deadline)
			if err != nil {
				log.Errorf("cannot clean old 30s episodes: %v", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func cleanOld5mEpisodes(ctx context.Context, dbCtx *dbcontext.DbContext, retDays int) {
	dayBack := -24 * time.Hour * time.Duration(retDays)
	period := 300 * time.Second

	conn := dbCtx.Start()
	defer conn.Stop()

	storage := dao.NewEpisodeDao5m(conn)

	interval := 24 * time.Hour
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Truncate(period).Add(dayBack)
			err := storage.DeleteUpTo(deadline)
			if err != nil {
				log.Errorf("cannot clean old 5m episodes: %v", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func initHttpServer(dbCtx *dbcontext.DbContext, downtimeMonitor *downtime.Monitor, controller *remotewrite.Controller, probeLister registry.ProbeLister, addr string) *http.Server {
	mux := http.NewServeMux()

	// API handlers
	mux.Handle("/api/probe", &api.ProbeListHandler{DbCtx: dbCtx, ProbeLister: probeLister})
	mux.Handle("/api/status/range", &api.StatusRangeHandler{DbCtx: dbCtx, DowntimeMonitor: downtimeMonitor})
	mux.Handle("/public/api/status", &api.PublicStatusHandler{DbCtx: dbCtx, DowntimeMonitor: downtimeMonitor, ProbeLister: probeLister})
	mux.Handle("/downtime", &api.AddEpisodesHandler{DbCtx: dbCtx, RemoteWrite: controller})
	mux.Handle("/stats", &api.StatsHandler{DbCtx: dbCtx})
	// Kubernetes probes
	mux.HandleFunc("/healthz", writeOk)
	mux.HandleFunc("/ready", writeOk)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server
}

func writeOk(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func initRemoteWriteController(ctx context.Context, dbCtx *dbcontext.DbContext, kubeClient kube.Client, originsCount int, logger *log.Logger, userAgent string) (*remotewrite.Controller, error) {
	config := &remotewrite.ControllerConfig{
		// collecting/exporting episodes as metrics
		Period: 2 * time.Second,
		// monitor configs in kubernetes
		Kubernetes: kubeClient,
		// read metrics and track exporter state in the DB
		DbCtx:        dbCtx,
		OriginsCount: originsCount,
		UserAgent:    userAgent,
		Logger:       logger,
	}
	controller := config.Controller()
	return controller, controller.Start(ctx)
}

func initDowntimeMonitor(ctx context.Context, kubeClient kube.Client, logger *log.Logger) (*downtime.Monitor, error) {
	m := downtime.NewMonitor(kubeClient, log.NewEntry(logger))
	return m, m.Start(ctx)
}

func newProbeLister(disabled []string, dynamic *DynamicProbesConfig) *registry.RegistryProbeLister {
	noLogger := newDummyLogger()
	noFilter := probe.NewProbeFilter(disabled)
	noAccess := kubernetes.FakeAccessor()
	dynamicConfig := probe.DynamicConfig{
		IngressNginxControllers: dynamic.IngressControllers,
		NodeGroups:              dynamic.NodeGroups,
	}
	dummyDoer := checker.NoopDoer{}
	runLoader := probe.NewLoader(noFilter, noAccess, nil, dynamicConfig, dummyDoer, noLogger)
	calcLoader := calculated.NewLoader(noFilter, noLogger)

	return registry.NewProbeLister(runLoader, calcLoader)
}

func newDummyLogger() *log.Logger {
	logger := log.New()
	logger.SetOutput(ioutil.Discard)
	return logger
}
