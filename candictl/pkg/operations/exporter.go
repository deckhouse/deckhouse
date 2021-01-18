package operations

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/config"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/cache"
)

type ConvergeExporter struct {
	kubeCl *client.KubernetesClient

	MetricsPath   string
	ListenAddress string
	CheckInterval time.Duration

	GaugeMetrics   map[string]*prometheus.GaugeVec
	CounterMetrics map[string]*prometheus.CounterVec
}

var (
	clusterStatuses      = []string{converge.OKStatus, converge.ErrorStatus, converge.ChangedStatus}
	nodeGroupStatuses    = []string{converge.OKStatus, converge.InsufficientStatus, converge.ExcessiveStatus}
	nodeTemplateStatuses = []string{converge.OKStatus, converge.AbsentStatus, converge.ChangedStatus}
	nodeStatuses         = []string{converge.OKStatus, converge.ErrorStatus, converge.ChangedStatus}
)

func NewConvergeExporter(address, path string, interval time.Duration) *ConvergeExporter {
	var sshClient *ssh.Client
	var err error
	if app.SSHHost != "" {
		sshClient, err = ssh.NewClientFromFlags().Start()
		if err != nil {
			panic(err)
		}
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
	c.getStatistic()

	ticker := time.NewTicker(c.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cache.ClearTemporaryDirs()
			c.getStatistic()
		case <-stopCh:
			log.ErrorLn("Stop exporter...")
			return
		}
	}
}

func (c *ConvergeExporter) getStatistic() {
	metaConfig, err := config.ParseConfigInCluster(c.kubeCl)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return
	}

	metaConfig.UUID, err = converge.GetClusterUUID(c.kubeCl)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
		return
	}

	statistic, err := converge.CheckState(c.kubeCl, metaConfig)
	if err != nil {
		log.ErrorLn(err)
		c.CounterMetrics["errors"].WithLabelValues().Inc()
	}

	for _, status := range clusterStatuses {
		if status == statistic.Cluster.Status {
			c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(1)
			continue
		}
		c.GaugeMetrics["cluster_status"].WithLabelValues(status).Set(0)
	}

	for _, ng := range statistic.NodeGroups {
		for _, status := range nodeGroupStatuses {
			if status == ng.Status {
				c.GaugeMetrics["node_group_status"].WithLabelValues(status, ng.Name).Set(1)
				continue
			}
			c.GaugeMetrics["node_group_status"].WithLabelValues(status, ng.Name).Set(0)
		}
	}

	for _, node := range statistic.Node {
		for _, status := range nodeStatuses {
			if status == node.Status {
				c.GaugeMetrics["node_status"].WithLabelValues(status, node.Group, node.Name).Set(1)
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
}
