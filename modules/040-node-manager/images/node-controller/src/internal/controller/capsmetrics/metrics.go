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

package capsmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const mdNameLabel = "machine_deployment_name"

var (
	replicasGauge    = newGauge("d8_caps_md_replicas", "Current replicas of a caps-controller MachineDeployment")
	desiredGauge     = newGauge("d8_caps_md_desired", "Desired replicas of a caps-controller MachineDeployment")
	readyGauge       = newGauge("d8_caps_md_ready", "Ready replicas of a caps-controller MachineDeployment")
	unavailableGauge = newGauge("d8_caps_md_unavailable", "Unavailable replicas of a caps-controller MachineDeployment")
	phaseGauge       = newGauge("d8_caps_md_phase", "Phase of a caps-controller MachineDeployment (1=Running,2=ScalingUp,3=ScalingDown,4=Failed,5=Unknown)")
)

func newGauge(name, help string) *prometheus.GaugeVec {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, []string{mdNameLabel})
	ctrlmetrics.Registry.MustRegister(g)
	return g
}

func setMachineDeploymentMetrics(name string, replicas, desired, ready, unavailable, phase float64) {
	labels := prometheus.Labels{mdNameLabel: name}
	replicasGauge.With(labels).Set(replicas)
	desiredGauge.With(labels).Set(desired)
	readyGauge.With(labels).Set(ready)
	unavailableGauge.With(labels).Set(unavailable)
	phaseGauge.With(labels).Set(phase)
}

func deleteMachineDeploymentMetrics(name string) {
	labels := prometheus.Labels{mdNameLabel: name}
	replicasGauge.Delete(labels)
	desiredGauge.Delete(labels)
	readyGauge.Delete(labels)
	unavailableGauge.Delete(labels)
	phaseGauge.Delete(labels)
}

func phaseValue(phase string) float64 {
	switch phase {
	case "Running":
		return 1
	case "ScalingUp":
		return 2
	case "ScalingDown":
		return 3
	case "Failed":
		return 4
	default:
		return 5
	}
}
