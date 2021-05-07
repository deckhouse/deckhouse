package server

import (
	"context"
	"fmt"
	"net/http"

	// Install default pprof endpoint.
	_ "net/http/pprof"
	"time"

	shapp "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"

	// Import sqlite3 driver.
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/app"
	"d8.io/upmeter/pkg/crd"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/db/migrations"
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
	dbPath          string
	dbMigrationPath string

	originsCount int
}

func New(originsCount int) *server {
	return &server{
		originsCount: originsCount,

		dbPath:          app.DatabasePath,
		dbMigrationPath: app.DatabaseMigrationsPath,
	}
}

func (s *server) Start(ctx context.Context) error {
	var err error

	logger := log.StandardLogger()

	kubeClient, err := initKubeClient()
	if err != nil {
		return fmt.Errorf("init kubernetes client: %v", err)
	}

	// Database connection with pool
	dbCtx, err := migrations.GetMigratedDatabase(s.dbPath, s.dbMigrationPath)
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	// Downtime CR monitor
	downtimeMonitor, err := initDowntimeMonitor(ctx, kubeClient)
	if err != nil {
		return fmt.Errorf("cannot start downtimes.deckhouse.io monitor: %v", err)
	}

	// Metrics controller
	controller, err := initMetricsController(ctx, dbCtx, kubeClient, s.originsCount, logger)
	if err != nil {
		logger.Debugf("starting controller... did't happen: %v", err)
		return fmt.Errorf("cannot start remote_write controller: %v", err)
	}

	go cleanOld30sEpisodes(ctx, dbCtx)

	// Start http server. It blocks, that's why it is the last here.
	logger.Debugf("starting HTTP server")
	err = serveHttp(dbCtx, downtimeMonitor, controller)
	return err
}

func cleanOld30sEpisodes(ctx context.Context, dbCtx *dbcontext.DbContext) {
	dayBack := -24 * time.Hour
	period := 30 * time.Second

	conn := dbCtx.Start()
	defer conn.Stop()

	storage := dao.NewEpisodeDao30s(conn)

	tick := time.NewTicker(period)
	for {
		select {
		case <-tick.C:
			deadline := time.Now().Truncate(period).Add(dayBack)
			err := storage.DeleteUpTo(deadline)
			if err != nil {
				log.Errorf("cannot clean old episodes: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func serveHttp(dbCtx *dbcontext.DbContext, downtimeMonitor *crd.DowntimeMonitor, controller *remotewrite.Controller) error {
	// Setup API handlers
	http.Handle("/api/probe", &api.ProbeListHandler{DbCtx: dbCtx})
	http.Handle("/api/status/range", &api.StatusRangeHandler{DbCtx: dbCtx, DowntimeMonitor: downtimeMonitor})
	http.Handle("/public/api/status", &api.PublicStatusHandler{DbCtx: dbCtx, DowntimeMonitor: downtimeMonitor})
	http.Handle("/downtime", &api.AddEpisodesHandler{DbCtx: dbCtx, RemoteWrite: controller})
	http.Handle("/stats", &api.StatsHandler{DbCtx: dbCtx})
	// Kubernetes probes
	http.HandleFunc("/healthz", writeOk)
	http.HandleFunc("/ready", writeOk)

	return http.ListenAndServe(app.ListenHost+":"+app.ListenPort, nil)
}

func writeOk(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func initMetricsController(ctx context.Context, dbCtx *dbcontext.DbContext, kubeClient kube.KubernetesClient, originsCount int, logger *log.Logger) (*remotewrite.Controller, error) {
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

func initKubeClient() (kube.KubernetesClient, error) {
	client := kube.NewKubernetesClient()

	client.WithContextName(shapp.KubeContext)
	client.WithConfigPath(shapp.KubeConfig)
	client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
	client.WithMetricStorage(metric_storage.NewMetricStorage())

	return client, client.Init()
}
