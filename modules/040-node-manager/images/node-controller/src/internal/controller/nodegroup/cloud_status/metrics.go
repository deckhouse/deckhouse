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

package cloud_status

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var machineDeploymentNodeGroupInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "machine_deployment_node_group_info",
		Help: "Info about machine deployments by node group",
	},
	[]string{"node_group", "name"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(machineDeploymentNodeGroupInfo)
}

func trackMachineDeploymentNodeGroupInfo(nodeGroup, name string) {
	machineDeploymentNodeGroupInfo.WithLabelValues(nodeGroup, name).Set(1)
}
