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

package fencingfailednodestate

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// configurationErrorGauge is the alertable counterpart of the ConfigurationError
// condition: while it reads 1 the node behind the incident is not evacuated, no
// matter how long ago its failure was detected.
var configurationErrorGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_configuration_error",
		Help: "Set to 1 while the SLA profile of a fencing incident cannot be resolved, which blocks evacuation of the node",
	},
	[]string{"node", "profile"},
)

// invalidNodeReferenceGauge is the alertable counterpart of the
// InvalidNodeReference condition: while it reads 1 the object does not identify
// a live node, so the node behind it is not evacuated. Unlike a broken profile
// this blocker is not retried and its event is published once, so the series is
// the only standing signal an operator gets.
var invalidNodeReferenceGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_invalid_node_reference",
		Help: "Set to 1 while a fencing incident does not identify the live node it names, which blocks evacuation of the node",
	},
	[]string{"node", "reason"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(configurationErrorGauge, invalidNodeReferenceGauge)
}

func reportConfigurationError(node string, profile v1alpha1.ProfileName) {
	configurationErrorGauge.WithLabelValues(node, string(profile)).Set(1)
}

// clearConfigurationError drops the series instead of zeroing it, so a node that
// recovered or left the cluster stops being reported at all.
func clearConfigurationError(node string) {
	configurationErrorGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}

func reportInvalidNodeReference(node, reason string) {
	// The reason is a label, so the previous one goes first: a node that moved
	// from one broken invariant to another must not report both at once.
	clearInvalidNodeReference(node)
	invalidNodeReferenceGauge.WithLabelValues(node, reason).Set(1)
}

// clearInvalidNodeReference drops the series instead of zeroing it, for the same
// reason clearConfigurationError does.
func clearInvalidNodeReference(node string) {
	invalidNodeReferenceGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}
