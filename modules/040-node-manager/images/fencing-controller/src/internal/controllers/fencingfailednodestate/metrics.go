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
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
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

// aliveAfterAttestationGauge reads 1 while the controller saw a heartbeat of the
// node after it saw the attestation. Such an attestation does not get the node
// the taint.
var aliveAfterAttestationGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_alive_after_attestation",
		Help: "Set to 1 while the node of a fencing incident sends heartbeats after its attestation, so the attestation does not get the node the out-of-service taint",
	},
	[]string{"node"},
)

// phaseGauge has one series for each incident: its phase and if its node is
// armed.
var phaseGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_failed_node_state_phase",
		Help: "Set to 1 for the current phase of the fencing incident of a node",
	},
	[]string{"node", "phase", "armed"},
)

// outOfServiceTaintedGauge reads 1 while the node has the out-of-service taint,
// no matter who set it.
var outOfServiceTaintedGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_node_out_of_service_tainted",
		Help: "Set to 1 while the node has the taint node.kubernetes.io/out-of-service",
	},
	[]string{"node"},
)

// notProgressingGauge reads 1 while the incident has the condition Progressing
// with the status False. The reason of the condition is a label.
var notProgressingGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_fencing_eviction_not_progressing",
		Help: "Set to 1 while the eviction of the node of a fencing incident is not complete in time, with the reason of the condition Progressing",
	},
	[]string{"node", "reason"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		configurationErrorGauge,
		aliveAfterAttestationGauge,
		phaseGauge,
		outOfServiceTaintedGauge,
		notProgressingGauge,
	)
}

// reportPhase keeps one series for the incident. An empty phase is reported as
// Suspected, because the controller did not process the object yet.
func reportPhase(incident *v1alpha1.FencingFailedNodeState) {
	phase := incident.Status.Phase
	if phase == "" {
		phase = v1alpha1.PhaseSuspected
	}

	// The phase is a label, so the series of the previous phase goes first.
	phaseGauge.DeletePartialMatch(prometheus.Labels{"node": incident.Name})
	phaseGauge.WithLabelValues(incident.Name, string(phase), strconv.FormatBool(incident.Spec.Armed)).Set(1)
}

// clearPhase drops the series of an incident that is deleted.
func clearPhase(node string) {
	phaseGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}

// reportOutOfServiceTaint publishes the series only while the node has the
// taint. Otherwise it drops the series.
func reportOutOfServiceTaint(node string, tainted bool) {
	if tainted {
		outOfServiceTaintedGauge.WithLabelValues(node).Set(1)

		return
	}

	outOfServiceTaintedGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}

// reportProgress publishes the series only while the incident has the
// condition Progressing with the status False.
func reportProgress(incident *v1alpha1.FencingFailedNodeState) {
	clearProgress(incident.Name)

	if progress := meta.FindStatusCondition(incident.Status.Conditions, common.ConditionTypeProgressing); progress != nil &&
		progress.Status == metav1.ConditionFalse {
		notProgressingGauge.WithLabelValues(incident.Name, progress.Reason).Set(1)
	}
}

func clearProgress(node string) {
	notProgressingGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}

func reportConfigurationError(node string, profile v1alpha1.ProfileName) {
	configurationErrorGauge.WithLabelValues(node, string(profile)).Set(1)
}

// clearConfigurationError drops the series instead of zeroing it, so a node that
// recovered or left the cluster stops being reported at all.
func clearConfigurationError(node string) {
	configurationErrorGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}

// reportAliveAfterAttestation publishes the series only while the node is alive
// after the attestation. Otherwise it drops the series.
func reportAliveAfterAttestation(node string, alive bool) {
	if alive {
		aliveAfterAttestationGauge.WithLabelValues(node).Set(1)

		return
	}

	clearAliveAfterAttestation(node)
}

func clearAliveAfterAttestation(node string) {
	aliveAfterAttestationGauge.DeletePartialMatch(prometheus.Labels{"node": node})
}
