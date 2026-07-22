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

package defaultcrimigration

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// obsoleteDefaultCRIGauge is set to 1 while `defaultCRI` is still configured in
// the deprecated ClusterConfiguration but has not been migrated to the
// node-manager ModuleConfig. The D8ObsoleteDefaultCRIInClusterConfiguration
// alert (see monitoring/prometheus-rules/mc-migration.yaml) fires on it.
var obsoleteDefaultCRIGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "d8_obsolete_default_cri_in_cluster_configuration",
		Help: "Set to 1 when defaultCRI is set to a non-default value in ClusterConfiguration but not migrated to the node-manager ModuleConfig",
	},
)

func init() {
	ctrlmetrics.Registry.MustRegister(obsoleteDefaultCRIGauge)
}

func setObsoleteDefaultCRIMetric(obsolete bool) {
	if obsolete {
		obsoleteDefaultCRIGauge.Set(1)
		return
	}
	obsoleteDefaultCRIGauge.Set(0)
}
