package nodetemplate

import (
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var unmanagedNodesGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_unmanaged_nodes_on_cluster",
		Help: "List of nodes without node.deckhouse.io/group label",
	},
	[]string{"node"},
)

var missingMasterTaintGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_nodegroup_taint_missing",
		Help: "Master nodegroup misses control-plane taint in non-single-node topology",
	},
	[]string{"name"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(unmanagedNodesGauge)
	ctrlmetrics.Registry.MustRegister(missingMasterTaintGauge)
}

func (r *Reconciler) syncUnmanagedNodesMetric(nodes []corev1.Node) {
	unmanagedNodesGauge.Reset()
	for i := range nodes {
		if nodes[i].Labels[nodeGroupNameLabel] == "" {
			unmanagedNodesGauge.WithLabelValues(nodes[i].Name).Set(1)
		}
	}
}

func (r *Reconciler) syncMissingMasterTaintMetric(nodeGroups []v1.NodeGroup, nodes []corev1.Node) {
	missingMasterTaintGauge.Reset()
	for i := range nodeGroups {
		if nodeGroups[i].Name != "master" {
			continue
		}
		if len(nodeGroups) == 1 && len(nodes) == 1 {
			return
		}

		controlPlaneTaintMissing := true
		for _, taint := range getTemplateTaints(&nodeGroups[i]) {
			if taint.Key == controlPlaneTaintKey {
				controlPlaneTaintMissing = false
				break
			}
		}
		if controlPlaneTaintMissing {
			missingMasterTaintGauge.WithLabelValues("master").Set(1)
		}
		return
	}
}
