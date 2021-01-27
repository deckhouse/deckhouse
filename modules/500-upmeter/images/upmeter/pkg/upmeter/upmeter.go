package upmeter

import (
	"context"
	"fmt"
	"net/http"
	// Install default pprof endpoint.
	_ "net/http/pprof"

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
)

// Informer initializes all dependencies:
// - kubernetes client
// - crd monitor
// - database connection
// - metrics storage
// If everything is ok, it starts http server.

type Informer struct {
	ctx    context.Context
	cancel context.CancelFunc

	DbPath string
	DbCtx  *dbcontext.DbContext

	KubernetesClient kube.KubernetesClient
	MetricStorage    *metric_storage.MetricStorage

	CrdMonitor *crd.Monitor
}

func NewInformer(ctx context.Context) *Informer {
	inf := &Informer{}
	inf.ctx, inf.cancel = context.WithCancel(ctx)
	return inf
}

func NewDefaultInformer(ctx context.Context) *Informer {
	inf := NewInformer(ctx)
	inf.DbPath = app.DowntimeDbPath

	// Metric storage
	inf.MetricStorage = metric_storage.NewMetricStorage()

	// Kubernetes client
	inf.KubernetesClient = kube.NewKubernetesClient()
	inf.KubernetesClient.WithContextName(shapp.KubeContext)
	inf.KubernetesClient.WithConfigPath(shapp.KubeConfig)
	inf.KubernetesClient.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
	inf.KubernetesClient.WithMetricStorage(inf.MetricStorage)

	return inf
}

func (inf *Informer) WithDbPath(path string) {
	inf.DbPath = path
}

func (inf *Informer) Start() error {
	var err error

	err = inf.KubernetesClient.Init()
	if err != nil {
		return fmt.Errorf("init kubernetes client: %v", err)
	}

	// CRD Monitor
	inf.CrdMonitor = crd.NewMonitor(inf.ctx)
	inf.CrdMonitor.Monitor.WithKubeClient(inf.KubernetesClient)
	err = inf.CrdMonitor.Start()
	if err != nil {
		return fmt.Errorf("start CRD monitor: %v", err)
	}

	// Setup db context with connection pool.
	inf.DbCtx, err = db.Connect(inf.DbPath)
	if err != nil {
		return fmt.Errorf("db connect with pool: %v", err)
	}

	// Apply migrations
	err = migrations.Migrator.Apply(inf.DbCtx)
	if err != nil {
		return fmt.Errorf("db migrate: %v", err)
	}

	// Setup API handlers
	probeListHandler := new(api.ProbeListHandler)
	probeListHandler.DbCtx = inf.DbCtx
	http.Handle("/api/probe", probeListHandler)

	statusHandler := new(api.StatusRangeHandler)
	statusHandler.CrdMonitor = inf.CrdMonitor
	statusHandler.DbCtx = inf.DbCtx
	http.Handle("/api/status/range", statusHandler)

	publicStatusHandler := new(api.PublicStatusHandler)
	publicStatusHandler.CrdMonitor = inf.CrdMonitor
	publicStatusHandler.DbCtx = inf.DbCtx
	http.Handle("/public/api/status", publicStatusHandler)

	downtimeHandler := new(api.DowntimeHandler)
	downtimeHandler.DbCtx = inf.DbCtx
	http.Handle("/downtime", downtimeHandler)

	statsHandler := new(api.StatsHandler)
	statsHandler.DbCtx = inf.DbCtx
	http.Handle("/stats", statsHandler)

	// Kubernetes probes
	http.HandleFunc("/healthz", inf.Healthz)
	http.HandleFunc("/ready", inf.Ready)

	// Start http server
	log.Fatal(http.ListenAndServe(app.UpmeterListenHost+":"+app.UpmeterListenPort, nil))
	return nil
}

func (inf *Informer) Stop() {
	if inf.cancel != nil {
		inf.cancel()
	}
}

func (inf *Informer) Healthz(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func (inf *Informer) Ready(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}
