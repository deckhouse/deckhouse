// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/name212/govalue"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

type previouslyExistedEntities struct {
	Nodes      map[string]string
	NodeGroups map[string]struct{}
}

func newPreviouslyExistedEntities() *previouslyExistedEntities {
	return &previouslyExistedEntities{Nodes: make(map[string]string), NodeGroups: make(map[string]struct{})}
}

func (p *previouslyExistedEntities) AddNode(name, nodeGroup string) {
	p.Nodes[name] = nodeGroup
}

func (p *previouslyExistedEntities) AddNodeGroup(name string) {
	p.NodeGroups[name] = struct{}{}
}

type ConvergeExporter struct {
	kubeCl                *client.KubernetesClient
	infrastructureContext *infrastructure.Context

	MetricsPath   string
	ListenAddress string
	CheckInterval time.Duration

	existedEntities *previouslyExistedEntities

	OneGaugeMetrics map[string]prometheus.Gauge
	GaugeMetrics    map[string]*prometheus.GaugeVec
	CounterMetrics  map[string]*prometheus.CounterVec

	tmpDir  string
	logger  log.Logger
	isDebug bool
}

var (
	clusterStatuses = []string{
		check.OKStatus,
		check.ErrorStatus,
		check.ChangedStatus,
		check.DestructiveStatus,
	}
	nodeStatuses = []string{
		check.OKStatus,
		check.ErrorStatus,
		check.ChangedStatus,
		check.DestructiveStatus,
		check.AbsentStatus,
		check.AbandonedStatus,
	}
	nodeTemplateStatuses = []string{
		check.OKStatus,
		check.ChangedStatus,
		check.AbsentStatus,
	}
)

type ExporterParams struct {
	Address  string
	Path     string
	Interval time.Duration
	TmpDir   string
	Logger   log.Logger
	IsDebug  bool
}

func NewConvergeExporter(params ExporterParams) *ConvergeExporter {
	sshClient, err := sshclient.NewInitClientFromFlags(true)
	if err != nil {
		panic(err)
	}

	kubeCl := client.NewKubernetesClient().WithNodeInterface(ssh.NewNodeInterfaceWrapper(sshClient))
	if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
		panic(err)
	}

	logger := params.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &ConvergeExporter{
		MetricsPath:           params.Path,
		ListenAddress:         params.Address,
		kubeCl:                kubeCl,
		infrastructureContext: infrastructure.NewContext(logger),
		CheckInterval:         params.Interval,

		existedEntities: newPreviouslyExistedEntities(),

		OneGaugeMetrics: make(map[string]prometheus.Gauge),
		GaugeMetrics:    make(map[string]*prometheus.GaugeVec),
		CounterMetrics:  make(map[string]*prometheus.CounterVec),
		tmpDir:          params.TmpDir,
		logger:          logger,
		isDebug:         params.IsDebug,
	}
}

func (c *ConvergeExporter) registerMetrics() {
	needMigrateToOpentofu := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "need_migrate_to_tofu",
		Help:      "Infrastructure states have terraform instead of opentofu",
	})
	prometheus.MustRegister(needMigrateToOpentofu)
	c.OneGaugeMetrics["need_migrate_to_tofu"] = needMigrateToOpentofu

	terraformStateVersionVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "terraform_state_version",
		Help:      "terraform version in the state",
	},
		[]string{"version"},
	)
	prometheus.MustRegister(terraformStateVersionVec)
	c.GaugeMetrics["terraform_state_version"] = terraformStateVersionVec

	clusterStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "cluster_status",
		Help:      "Infrastructure state status of Kubernetes cluster",
	},
		[]string{"status"},
	)
	prometheus.MustRegister(clusterStateVec)
	c.GaugeMetrics["cluster_status"] = clusterStateVec

	nodeGroupStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "node_group_status",
		Help:      "Infrastructure state status of NodeGroup",
	},
		[]string{"status", "name"},
	)
	prometheus.MustRegister(nodeGroupStateVec)
	c.GaugeMetrics["node_group_status"] = nodeGroupStateVec

	nodeStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "node_status",
		Help:      "Infrastructure state status of single Node",
	},
		[]string{"status", "node_group", "name"},
	)
	prometheus.MustRegister(nodeStateVec)
	c.GaugeMetrics["node_status"] = nodeStateVec

	nodeTemplateStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "node_template_status",
		Help:      "State of nodeTemplate settings of NodeGroup",
	},
		[]string{"status", "name"},
	)
	prometheus.MustRegister(nodeTemplateStateVec)
	c.GaugeMetrics["node_template_status"] = nodeTemplateStateVec

	errorsVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "exporter_errors",
		Help:      "Errors of converge operations",
	},
		[]string{},
	)
	prometheus.MustRegister(errorsVec)
	c.CounterMetrics["errors"] = errorsVec
}

func (c *ConvergeExporter) Start(ctx context.Context) {
	log.InfoLn("Start exporter")
	log.InfoLn("Address: ", app.ListenAddress)
	log.InfoLn("Metrics path: ", app.MetricsPath)
	log.InfoLn("Checks interval: ", app.CheckInterval)
	c.registerMetrics()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.convergeLoop(ctx)

	http.Handle(c.MetricsPath, promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
             <head><title>CandI Converge Exporter</title></head>
             <body>
             <h1>CandI Converge Exporter</h1>
             <p><a href='` + c.MetricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	if err := http.ListenAndServe(c.ListenAddress, nil); err != nil {
		log.ErrorF("Error starting HTTP server: %v\n", err)
		cancel()
		os.Exit(1)
	}
}

func (c *ConvergeExporter) convergeLoop(ctx context.Context) {
	clearTmp := cache.GetClearTemporaryDirsFunc(cache.ClearTmpParams{
		IsDebug:         c.isDebug,
		RemoveTombStone: true,
		TmpDir:          c.tmpDir,
		DefaultTmpDir:   c.tmpDir, // do not remove root tmp dir
		LoggerProvider: func() log.Logger {
			return c.logger
		},
	})

	c.recordStatistic(c.getStatistic(ctx, clearTmp))

	ticker := time.NewTicker(c.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.recordStatistic(c.getStatistic(ctx, clearTmp))
		case <-ctx.Done():
			log.ErrorLn("Stop exporter...")
			return
		}
	}
}

func (c *ConvergeExporter) getStatistic(ctx context.Context, clearTmp func()) (*check.Statistics, bool) {
	metaConfig, err := config.ParseConfigInCluster(
		ctx,
		c.kubeCl,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(c.logger),
		),
	)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil, false
	}

	metaConfig.UUID, err = infrastructurestate.GetClusterUUID(ctx, c.kubeCl)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil, false
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           c.tmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           c.logger,
		IsDebug:          c.isDebug,
	})

	provider, err := providerGetter(ctx, metaConfig)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil, false
	}

	defer func() {
		err := provider.Cleanup()
		if err != nil {
			c.logger.LogErrorF("Cannot cleanup provider after getting statistic: %v\n", err)
			c.CounterMetrics["errors"].WithLabelValues().Inc()
		}

		clearTmp()
	}()

	c.infrastructureContext.SetCloudProviderGetter(providerGetter)

	statistic, hasTerraformState, err := check.CheckState(ctx, c.kubeCl, metaConfig, c.infrastructureContext, check.CheckStateOptions{})
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()

		// We still want to return collected statistic in case of error, because the error returned from
		// the CheckState call is a combination of errors from all infrastructure utility runs.
	}

	if !provider.NeedToUseTofu() {
		hasTerraformState = false
	}

	return statistic, hasTerraformState
}

func (c *ConvergeExporter) recordStatistic(statistic *check.Statistics, hasTerraformState bool) {
	if statistic == nil {
		return
	}

	c.OneGaugeMetrics["need_migrate_to_tofu"].Set(0)

	if hasTerraformState {
		c.OneGaugeMetrics["need_migrate_to_tofu"].Set(1)
	}

	for _, status := range clusterStatuses {
		if status == statistic.Cluster.Status {
			c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(1)
			log.InfoF("Cluster status is %s\n", status)
			continue
		}

		c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(0)
		log.InfoF("Cluster status: clean %s status\n", status)
	}

	newExistedEntities := newPreviouslyExistedEntities()

	for _, node := range statistic.Node {
		for _, status := range nodeStatuses {
			if status == node.Status {
				c.GaugeMetrics["node_status"].WithLabelValues(status, node.Group, node.Name).Set(1)
				newExistedEntities.AddNode(node.Name, node.Group)
				log.InfoF("%v/%v: node status is %v\n", node.Group, node.Name, status)
				continue
			}
			c.GaugeMetrics["node_status"].WithLabelValues(status, node.Group, node.Name).Set(0)
			log.InfoF("%v/%v: clean node status %v\n", node.Group, node.Name, status)
		}
	}

	for _, template := range statistic.NodeTemplates {
		for _, status := range nodeTemplateStatuses {
			if status == template.Status {
				c.GaugeMetrics["node_template_status"].WithLabelValues(status, template.Name).Set(1)
				log.InfoF("%v: node template status is %v\n", template.Name, status)
				continue
			}
			c.GaugeMetrics["node_template_status"].WithLabelValues(status, template.Name).Set(0)
			log.InfoF("%v: node template clean status %v\n", template.Name, status)
		}
	}

	// mark missed nodes and node groups statuses as 0 to avoid incorrect statistic
	for nodeName, nodeGroup := range c.existedEntities.Nodes {
		if _, ok := newExistedEntities.Nodes[nodeName]; ok {
			continue
		}
		for _, status := range nodeStatuses {
			c.GaugeMetrics["node_status"].WithLabelValues(status, nodeGroup, nodeName).Set(0)
			log.InfoF("%v/%v: clean missing node status %v\n", nodeGroup, nodeName, status)
		}
	}

	// getStatistic is executed in a loop by timer, so no race condition here
	c.existedEntities = newExistedEntities
}
