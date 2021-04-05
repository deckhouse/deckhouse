package upmeter

import (
	"context"
	"fmt"
	"net/http"
	// Install default pprof endpoint.
	_ "net/http/pprof"
	"os"
	"time"

	shapp "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	_ "github.com/mattn/go-sqlite3" // Import sqlite3 driver.
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/app"
	"upmeter/pkg/crd"
	"upmeter/pkg/upmeter/api"
	"upmeter/pkg/upmeter/db"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/migrations"
	"upmeter/pkg/upmeter/remotewrite"
)

// Informer initializes all dependencies:
// - kubernetes client
// - crd monitor
// - database connection
// - metrics storage
// If everything is ok, it starts http server.

type Informer struct {
	kubernetesClient kube.KubernetesClient

	dbPath string
	dbCtx  *dbcontext.DbContext

	originsCount          int
	remoteWriteController *remotewrite.Controller
	downtimeMonitor       *crd.DowntimeMonitor

	cancel context.CancelFunc
}

func NewInformer(originsCount int) *Informer {
	kubeClient := kube.NewKubernetesClient()
	kubeClient.WithContextName(shapp.KubeContext)
	kubeClient.WithConfigPath(shapp.KubeConfig)
	kubeClient.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
	kubeClient.WithMetricStorage(metric_storage.NewMetricStorage())

	return &Informer{
		kubernetesClient: kubeClient,

		dbPath:       app.DowntimeDbPath,
		originsCount: originsCount,
	}
}

func (inf *Informer) WithDbPath(path string) {
	inf.dbPath = path
}

func (inf *Informer) Start(ctx context.Context) error {
	ctx, inf.cancel = context.WithCancel(ctx)
	var err error

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.ErrorLevel
	}
	log.SetLevel(logLevel)

	err = inf.kubernetesClient.Init()
	if err != nil {
		return fmt.Errorf("init kubernetes client: %v", err)
	}

	// Setup db context with connection pool.
	inf.dbCtx, err = db.Connect(inf.dbPath)
	if err != nil {
		return fmt.Errorf("cannot connect to database with pool: %v", err)
	}

	// Apply migrations
	err = migrations.Migrator.Apply(inf.dbCtx)
	if err != nil {
		return fmt.Errorf("cannot migrate database: %v", err)
	}

	// Downtime CR  configMonitor
	inf.downtimeMonitor = crd.NewMonitor(ctx)
	inf.downtimeMonitor.Monitor.WithKubeClient(inf.kubernetesClient)
	err = inf.downtimeMonitor.Start()
	if err != nil {
		return fmt.Errorf("cannot start downtimes.deckhouse.io monitor: %v", err)
	}

	logger := log.StandardLogger()

	// Metrics controller
	logger.Debugf("creating controller")
	config := &remotewrite.ControllerConfig{
		// collecting/exporting episodes as metrics
		Period: 2 * time.Second,
		// monitor configs in kubernetes
		Kubernetes: inf.kubernetesClient,
		// read metrics and track exporter state in the DB
		DbCtx:        inf.dbCtx,
		OriginsCount: inf.originsCount,
		Logger:       logger,
	}
	controller := config.Controller()
	logger.Debugf("starting controller")
	err = controller.Start(ctx)
	if err != nil {
		logger.Debugf("starting controller... did't happen: %v", err)
		return fmt.Errorf("cannot start remote_write controller: %v", err)
	}

	// Setup API handlers
	http.Handle("/api/probe", &api.ProbeListHandler{DbCtx: inf.dbCtx})
	http.Handle("/api/status/range", &api.StatusRangeHandler{DbCtx: inf.dbCtx, DowntimeMonitor: inf.downtimeMonitor})
	http.Handle("/public/api/status", &api.PublicStatusHandler{DbCtx: inf.dbCtx, DowntimeMonitor: inf.downtimeMonitor})
	http.Handle("/downtime", &api.DowntimeHandler{DbCtx: inf.dbCtx, RemoteWrite: controller})
	http.Handle("/stats", &api.StatsHandler{DbCtx: inf.dbCtx})

	// Kubernetes probes
	http.HandleFunc("/healthz", writeOk)
	http.HandleFunc("/ready", writeOk)

	logger.Debugf("starting HTTP server")

	// Start http server. It blocks, that's why it is the last here.
	err = http.ListenAndServe(app.UpmeterListenHost+":"+app.UpmeterListenPort, nil)
	if err != nil {
		return err
	}

	return nil
}

func (inf *Informer) Stop() {
	if inf.cancel == nil {
		return
	}
	inf.cancel()
	inf.downtimeMonitor.Stop()
	inf.remoteWriteController.Stop()

}

func writeOk(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}
