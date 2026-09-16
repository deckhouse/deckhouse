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

package derived_status

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// Reason label values for preemptibleUnsupportedGauge.
const (
	preemptibleUnsupportedReasonMCM         = "mcm"
	preemptibleUnsupportedReasonNonSelectel = "non-selectel"
)

// preemptibleUnsupportedGauge fires per NodeGroup whose OpenStackInstanceClass sets
// spec.preemptible: true but where the field cannot reach Nova as a working preemption tag.
// The provider template silently drops the tag in that case (see capi/template.yaml), and this
// gauge is what the OpenStackPreemptibleUnsupported alert reads to explain the situation.
//
// The metric is 0 for the healthy case (the label pair is deleted, so no timeseries at all)
// and 1 for the unhealthy one — a gauge rather than an info metric because Prometheus alerting
// wants numeric comparisons and the label pair uniquely identifies the state.
var preemptibleUnsupportedGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_openstack_preemptible_unsupported",
		Help: "OpenStackInstanceClass.spec.preemptible: true is set on a NodeGroup where the tag cannot reach Nova (MCM engine, or non-Selectel provider).",
	},
	[]string{"node_group", "reason"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(preemptibleUnsupportedGauge)
}

// SyncPreemptibleUnsupported publishes the metric for a single NodeGroup. Called from
// ResolveNodeGroup so every reconcile refreshes the state; idempotent per (nodeGroup, reason)
// pair. When the situation no longer applies the previous label pair is cleared, so the alert
// stops firing without waiting for scrape rotation.
//
// Callers must invoke DeletePreemptibleUnsupported on NodeGroup deletion — otherwise a deleted
// NG keeps its last recorded label pair until controller restart.
func SyncPreemptibleUnsupported(ng *v1.NodeGroup, snap Snapshot) {
	// Non-openstack NGs never touch this metric — nothing to clean, nothing to set.
	if ng.Spec.CloudInstances == nil || ng.Spec.CloudInstances.ClassReference.Kind != "OpenStackInstanceClass" {
		return
	}
	// The metric always mirrors the current state, so wipe any previous label pair for this NG
	// before deciding whether a new one applies. A NG that fixed its config transitions to 0
	// timeseries, which lets the alert clear on the next scrape.
	preemptibleUnsupportedGauge.DeletePartialMatch(prometheus.Labels{"node_group": ng.Name})

	preemptible, _ := snap.InstanceClass["preemptible"].(bool)
	if !preemptible {
		return
	}

	// MCM engine — the raw Nova tag has no MCM analogue at all (no field in MachineClass maps to
	// it), so the render silently drops it. The operator asked for a preemptible NG on MCM; the
	// alert tells them to migrate to CAPI. Checked first because engineMCM overrides any authURL.
	if snap.Engine == engineMCM {
		preemptibleUnsupportedGauge.WithLabelValues(ng.Name, preemptibleUnsupportedReasonMCM).Set(1)
		return
	}

	// CAPI on a provider that isn't Selectel — the tag is emitted by the template only when the
	// cluster's connection.authURL matches Selectel (capi/template.yaml). Off Selectel Nova
	// accepts the tag but ignores it, so the operator's request for preemption never materialises.
	if snap.Engine == engineCAPI && preemptibleEmissionSuppressed(snap.Provider) {
		preemptibleUnsupportedGauge.WithLabelValues(ng.Name, preemptibleUnsupportedReasonNonSelectel).Set(1)
	}
}

// DeletePreemptibleUnsupported drops any label pair for a NodeGroup that no longer exists.
// Called from the NodeGroup Status reconciler on IsNotFound so the alert clears at NG-delete
// time instead of hanging around until controller restart.
func DeletePreemptibleUnsupported(nodeGroupName string) {
	preemptibleUnsupportedGauge.DeletePartialMatch(prometheus.Labels{"node_group": nodeGroupName})
}

// preemptibleEmissionSuppressed mirrors the template's authURL gate: a missing authURL during
// bootstrap is treated as "not yet Selectel", not "not Selectel", so the metric doesn't spike
// on the first reconcile before the discovery hook publishes the connection block.
func preemptibleEmissionSuppressed(reg CloudProviderRegistration) bool {
	authURL := selectelAuthURLOf(reg)
	if authURL == "" {
		return false
	}
	lc := strings.ToLower(authURL)
	return !strings.Contains(lc, "selcloud.ru") && !strings.Contains(lc, "selectel")
}

func selectelAuthURLOf(reg CloudProviderRegistration) string {
	connection, ok := reg.CloudVariables["connection"].(map[string]any)
	if !ok {
		return ""
	}
	authURL, _ := connection["authURL"].(string)
	return authURL
}
