package operations

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
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
	kubeCl *client.KubernetesClient

	MetricsPath   string
	ListenAddress string
	CheckInterval time.Duration

	existedEntities *previouslyExistedEntities

	GaugeMetrics   map[string]*prometheus.GaugeVec
	CounterMetrics map[string]*prometheus.CounterVec
}

var (
	clusterStatuses = []string{
		converge.OKStatus,
		converge.ErrorStatus,
		converge.ChangedStatus,
		converge.DestructiveStatus,
	}
	nodeStatuses = []string{
		converge.OKStatus,
		converge.ErrorStatus,
		converge.ChangedStatus,
		converge.DestructiveStatus,
		converge.AbsentStatus,
		converge.AbandonedStatus,
	}
	nodeTemplateStatuses = []string{
		converge.OKStatus,
		converge.ChangedStatus,
		converge.AbsentStatus,
	}
)

func NewConvergeExporter(address, path string, interval time.Duration) *ConvergeExporter {
	sshClient, err := ssh.NewInitClientFromFlags(false)
	if err != nil {
		panic(err)
	}

	kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
	if err := kubeCl.Init(); err != nil {
		panic(err)
	}

	return &ConvergeExporter{
		MetricsPath:   path,
		ListenAddress: address,
		kubeCl:        kubeCl,
		CheckInterval: interval,

		existedEntities: newPreviouslyExistedEntities(),

		GaugeMetrics:   make(map[string]*prometheus.GaugeVec),
		CounterMetrics: make(map[string]*prometheus.CounterVec),
	}
}

func (c *ConvergeExporter) registerMetrics() {
	clusterStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "cluster_status",
		Help:      "Terraform state status of Kubernetes cluster",
	},
		[]string{"status"},
	)
	prometheus.MustRegister(clusterStateVec)
	c.GaugeMetrics["cluster_status"] = clusterStateVec

	nodeGroupStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "node_group_status",
		Help:      "Terraform state status of NodeGroup",
	},
		[]string{"status", "name"},
	)
	prometheus.MustRegister(nodeGroupStateVec)
	c.GaugeMetrics["node_group_status"] = nodeGroupStateVec

	nodeStateVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "candi",
		Subsystem: "converge",
		Name:      "node_status",
		Help:      "Terraform state status of single Node",
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

func (c *ConvergeExporter) Start() {
	log.InfoLn("Start exporter")
	log.InfoLn("Address: ", app.ListenAddress)
	log.InfoLn("Metrics path: ", app.MetricsPath)
	log.InfoLn("Checks interval: ", app.CheckInterval)
	c.registerMetrics()

	stopCh := make(chan struct{})
	go c.convergeLoop(stopCh)

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
		stopCh <- struct{}{}
		os.Exit(1)
	}
}

func (c *ConvergeExporter) convergeLoop(stopCh chan struct{}) {
	c.recordStatistic(c.getStatistic())

	ticker := time.NewTicker(c.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cache.ClearTemporaryDirs()
			c.recordStatistic(c.getStatistic())
		case <-stopCh:
			log.ErrorLn("Stop exporter...")
			return
		}
	}
}

func (c *ConvergeExporter) getStatistic() *converge.Statistics {
	metaConfig, err := config.ParseConfigInCluster(c.kubeCl)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil
	}

	metaConfig.UUID, err = converge.GetClusterUUID(c.kubeCl)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil
	}

	statistic, err := converge.CheckState(c.kubeCl, metaConfig)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return nil
	}

	return statistic
}

func (c *ConvergeExporter) recordStatistic(statistic *converge.Statistics) {
	if statistic == nil {
		return
	}

	for _, status := range clusterStatuses {
		if status == statistic.Cluster.Status {
			c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(1)
			continue
		}
		c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(0)
	}

	newExistedEntities := newPreviouslyExistedEntities()

	for _, node := range statistic.Node {
		for _, status := range nodeStatuses {
			if status == node.Status {
				c.GaugeMetrics["node_status"].WithLabelValues(status, node.Group, node.Name).Set(1)
				newExistedEntities.AddNode(node.Name, node.Group)
				continue
			}
			c.GaugeMetrics["node_status"].WithLabelValues(status, node.Group, node.Name).Set(0)
		}
	}

	for _, template := range statistic.NodeTemplates {
		for _, status := range nodeTemplateStatuses {
			if status == template.Status {
				c.GaugeMetrics["node_template_status"].WithLabelValues(status, template.Name).Set(1)
				continue
			}
			c.GaugeMetrics["node_template_status"].WithLabelValues(status, template.Name).Set(0)
		}
	}

	// mark missed nodes and node groups statuses as 0 to avoid incorrect statistic
	for nodeName, nodeGroup := range c.existedEntities.Nodes {
		if _, ok := newExistedEntities.Nodes[nodeName]; ok {
			continue
		}
		for _, status := range nodeStatuses {
			c.GaugeMetrics["node_status"].WithLabelValues(status, nodeGroup, nodeName).Set(0)
		}
	}

	// getStatistic is executed in a loop by timer, so no race condition here
	c.existedEntities = newExistedEntities
}
