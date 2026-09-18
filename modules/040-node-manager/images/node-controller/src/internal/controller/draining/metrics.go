/*
Copyright 2026 Flant JSC

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

package draining

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// nodeDrainingGauge carries no error message: the message is durable on the Node
// itself now, and as a label it would put a whole error string — pod names, a
// timeout — into the series identity. Every retry that phrases its failure
// differently would then start a fresh series, and NodeStuckInDraining, which
// groups by the same labels, would restart its `for: 5m` instead of firing.
var nodeDrainingGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_node_draining",
		Help: "Set to 1 while the node carries a failed drain, see the drain-failed annotation for the error",
	},
	[]string{"node"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(nodeDrainingGauge)
}

func clearDrainMetric(nodeName string) {
	nodeDrainingGauge.DeleteLabelValues(nodeName)
}

// syncDrainMetric raises the gauge for a node carrying the drain-failed marker,
// which is how a restart gets its gauges back: the annotation is all that is left
// of a failure the process did not see.
//
// It only ever sets. Clearing belongs to the two passes that actually resolve a
// failure — a drain that succeeds, a request that is withdrawn — because the
// retry's own pass may read the node from a cache older than the marker, and
// clearing on that would drop the gauge for the whole of the next attempt.
// Setting a series that already exists is idempotent, so no scrape sees a gap.
func syncDrainMetric(nodeName string, annotations map[string]string) {
	if _, failed := annotations[nodecommon.DrainFailedAnnotation]; failed {
		nodeDrainingGauge.WithLabelValues(nodeName).Set(1)
	}
}
