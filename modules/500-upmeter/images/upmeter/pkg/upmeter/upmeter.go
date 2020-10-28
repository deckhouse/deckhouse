package upmeter

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	shapp "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/app"
	"upmeter/pkg/crd"
	"upmeter/pkg/upmeter/api"
	"upmeter/pkg/upmeter/db"
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

	// Setup DB connect
	err = db.Connect(inf.DbPath)
	if err != nil {
		return fmt.Errorf("db connect: %v", err)
	}

	// Apply migrations
	err = migrations.Migrator.Apply()
	if err != nil {
		return fmt.Errorf("db migrate: %v", err)
	}

	// Setup API handlers
	http.HandleFunc("/api/probe", api.ProbeListHandler)

	statusHandler := api.NewStatusRangeHandler()
	statusHandler.WithCRDMonitor(inf.CrdMonitor)
	http.Handle("/api/status/range", statusHandler)

	publicStatusHandler := api.NewPublicStatusHandler()
	publicStatusHandler.WithCRDMonitor(inf.CrdMonitor)
	http.Handle("/public/api/status", publicStatusHandler)

	http.HandleFunc("/downtime", api.DowntimeHandler)

	http.HandleFunc("/stats", api.Stats)

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
