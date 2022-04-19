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

	"github.com/flant/shell-operator/pkg/kube"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/crd"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
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

type server struct {
	config *Config

	logger *log.Logger

	server                *http.Server
	downtimeMonitor       *crd.DowntimeMonitor
	remoteWriteController *remotewrite.Controller
}

type Config struct {
	ListenHost string
	ListenPort string
	UserAgent  string

	DatabasePath string

	OriginsCount   int
	DisabledProbes []string
}

func New(config *Config, logger *log.Logger) *server {
	return &server{
		config: config,
		logger: logger,
	}
}

func (s *server) Start(ctx context.Context) error {
	var err error

	kubeClient, err := kubernetes.InitKubeClient()
	if err != nil {
		return fmt.Errorf("init kubernetes client: %v", err)
	}

	// Database connection with pool
	dbctx, err := db.Connect(s.config.DatabasePath, dbcontext.DefaultConnectionOptions())
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	// Downtime CR monitor
	s.downtimeMonitor, err = initDowntimeMonitor(ctx, kubeClient)
	if err != nil {
		return fmt.Errorf("cannot start downtimes.deckhouse.io monitor: %v", err)
	}

	// Metrics controller
	s.remoteWriteController, err = initRemoteWriteController(ctx, dbctx, kubeClient, s.config.OriginsCount, s.logger)
	if err != nil {
		s.logger.Debugf("starting controller... did't happen: %v", err)
		return fmt.Errorf("cannot start remote_write controller: %v", err)
	}

	go cleanOld30sEpisodes(ctx, dbctx)

	// Probe probeLister that can only list groups and probes
	probeLister := newProbeLister(s.config.DisabledProbes)

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

func (s *server) Stop() error {
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
				log.Errorf("cannot clean old episodes: %v", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func initHttpServer(dbCtx *dbcontext.DbContext, downtimeMonitor *crd.DowntimeMonitor, controller *remotewrite.Controller, probeLister registry.ProbeLister, addr string) *http.Server {
	mux := http.NewServeMux()

	// Setup API handlers
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

func initRemoteWriteController(ctx context.Context, dbCtx *dbcontext.DbContext, kubeClient kube.KubernetesClient, originsCount int, logger *log.Logger) (*remotewrite.Controller, error) {
	config := &remotewrite.ControllerConfig{
		// collecting/exporting episodes as metrics
		Period: 2 * time.Second,
		// monitor configs in kubernetes
		Kubernetes: kubeClient,
		// read metrics and track exporter state in the DB
		DbCtx:        dbCtx,
		OriginsCount: originsCount,
		Logger:       logger,
	}
	controller := config.Controller()
	return controller, controller.Start(ctx)
}

func initDowntimeMonitor(ctx context.Context, kubeClient kube.KubernetesClient) (*crd.DowntimeMonitor, error) {
	m := crd.NewMonitor(ctx)
	m.Monitor.WithKubeClient(kubeClient)
	return m, m.Start()
}

func newProbeLister(disabled []string) *registry.RegistryProbeLister {
	noLogger := newDummyLogger()
	noFilter := probe.NewProbeFilter(disabled)
	noAccess := &kubernetes.Accessor{}

	runLoader := probe.NewLoader(noFilter, noAccess, noLogger)
	calcLoader := calculated.NewLoader(noFilter, noLogger)

	return registry.NewProbeLister(runLoader, calcLoader)
}

func newDummyLogger() *log.Logger {
	logger := log.New()
	logger.SetOutput(ioutil.Discard)
	return logger
}
