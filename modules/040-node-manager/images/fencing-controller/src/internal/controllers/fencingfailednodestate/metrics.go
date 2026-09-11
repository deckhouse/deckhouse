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

func init() {
	ctrlmetrics.Registry.MustRegister(configurationErrorGauge)
}

func reportConfigurationError(node string, profile v1alpha1.ProfileName) {
	configurationErrorGauge.WithLabelValues(node, string(profile)).Set(1)
}

// clearConfigurationError drops the series instead of zeroing it, so a node that
// recovered or left the cluster stops being reported at all.
func clearConfigurationError(node string) {
	configurationErrorGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}
