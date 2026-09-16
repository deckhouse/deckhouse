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
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The gauge is a package-level singleton, so tests share it. Each case wipes both possible label
// pairs for its NG before publishing to avoid leftover state from a previous case in the file.
func resetPreemptibleGaugeFor(nodeGroup string) {
	DeletePreemptibleUnsupported(nodeGroup)
}

func openstackPreemptibleMetricNG(name string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "OpenStackInstanceClass", Name: name},
			},
		},
	}
}

func snapshotWithProvider(engine, authURL string, preemptible bool) Snapshot {
	snap := Snapshot{
		Provider: CloudProviderRegistration{
			Type:                    "openstack",
			InstanceClassKind:       "OpenStackInstanceClass",
			InstanceClassAPIVersion: "v1alpha1",
		},
		Engine:        engine,
		InstanceClass: map[string]any{"preemptible": preemptible},
	}
	if authURL != "" {
		snap.Provider.CloudVariables = map[string]any{
			"connection": map[string]any{"authURL": authURL},
		}
	}
	return snap
}

func metricValueFor(t *testing.T, nodeGroup, reason string) float64 {
	t.Helper()
	return testutil.ToFloat64(preemptibleUnsupportedGauge.WithLabelValues(nodeGroup, reason))
}

// The alert fires on MCM+preemptible regardless of authURL: no MCM MachineClass field maps to a
// raw Nova tag, so the request never reaches Nova at all.
func TestSyncPreemptibleUnsupported_MCMEmitsMCMReason(t *testing.T) {
	const ng = "test-mcm-emits"
	resetPreemptibleGaugeFor(ng)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineMCM, "https://cloud.api.selcloud.ru/identity/v3", true))

	assert.Equal(t, 1.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// CAPI + a Selectel-looking authURL is the healthy case — no timeseries at all.
func TestSyncPreemptibleUnsupported_CAPISelectelSelcloudEmitsNothing(t *testing.T) {
	const ng = "test-capi-selcloud"
	resetPreemptibleGaugeFor(ng)
	// Prime with a stale value so we can prove the sync wipes it on the healthy transition.
	preemptibleUnsupportedGauge.WithLabelValues(ng, preemptibleUnsupportedReasonNonSelectel).Set(1)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineCAPI, "https://cloud.api.selcloud.ru/identity/v3", true))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

func TestSyncPreemptibleUnsupported_CAPISelectelDomainEmitsNothing(t *testing.T) {
	const ng = "test-capi-selectel-domain"
	resetPreemptibleGaugeFor(ng)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineCAPI, "https://api.selectel.ru/identity/v3", true))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// CAPI + non-Selectel authURL is the silent-drop case the alert exists to warn about.
func TestSyncPreemptibleUnsupported_CAPINonSelectelEmitsNonSelectelReason(t *testing.T) {
	const ng = "test-capi-nonselectel"
	resetPreemptibleGaugeFor(ng)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineCAPI, "https://public.infra.mail.ru:5000/v3/", true))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 1.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// authURL missing during bootstrap is treated as "not yet Selectel" — the alert must not fire on
// the first reconcile before the discovery hook publishes the connection block.
func TestSyncPreemptibleUnsupported_MissingAuthURLIsQuiet(t *testing.T) {
	const ng = "test-missing-authurl"
	resetPreemptibleGaugeFor(ng)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineCAPI, "", true))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// preemptible: false — the operator has not asked for preemption. Any stale label pair from a
// previous "true" state is cleared here so the alert stops firing on the next scrape.
func TestSyncPreemptibleUnsupported_PreemptibleFalseClearsStale(t *testing.T) {
	const ng = "test-preempt-false"
	resetPreemptibleGaugeFor(ng)
	preemptibleUnsupportedGauge.WithLabelValues(ng, preemptibleUnsupportedReasonMCM).Set(1)

	SyncPreemptibleUnsupported(openstackPreemptibleMetricNG(ng),
		snapshotWithProvider(engineMCM, "", false))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// Non-openstack NG must not be touched by this metric even when the surrounding shape would
// otherwise match — the field is provider-scoped, and a foreign kind should never appear in
// the alert.
func TestSyncPreemptibleUnsupported_NonOpenstackKindSkipped(t *testing.T) {
	const ng = "test-yandex-skipped"
	resetPreemptibleGaugeFor(ng)

	yaNG := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: ng},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "YandexInstanceClass", Name: ng},
			},
		},
	}
	SyncPreemptibleUnsupported(yaNG,
		snapshotWithProvider(engineMCM, "https://cloud.api.selcloud.ru/identity/v3", true))

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// DeletePreemptibleUnsupported wipes both possible label pairs — used by the Status reconciler
// on NG delete so the alert clears immediately rather than hanging until the next controller
// restart.
func TestDeletePreemptibleUnsupported_WipesBothReasons(t *testing.T) {
	const ng = "test-delete-wipes"
	resetPreemptibleGaugeFor(ng)
	preemptibleUnsupportedGauge.WithLabelValues(ng, preemptibleUnsupportedReasonMCM).Set(1)
	preemptibleUnsupportedGauge.WithLabelValues(ng, preemptibleUnsupportedReasonNonSelectel).Set(1)

	DeletePreemptibleUnsupported(ng)

	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonMCM))
	assert.Equal(t, 0.0, metricValueFor(t, ng, preemptibleUnsupportedReasonNonSelectel))
}

// The reason label values are part of the alert's PromQL branching (`{{ if $labels.reason == "mcm" }}`),
// so a rename of the constant here without updating the alert breaks the runbook. Freeze the
// exact strings the alert reads.
func TestPreemptibleUnsupportedReasons_FreezeLabelValues(t *testing.T) {
	assert.Equal(t, "mcm", preemptibleUnsupportedReasonMCM,
		"the alert PromQL branches on this literal — update the alert template if you change it")
	assert.Equal(t, "non-selectel", preemptibleUnsupportedReasonNonSelectel,
		"the alert PromQL branches on this literal — update the alert template if you change it")
	// Belt-and-braces: the metric name is also referenced by the alert.
	assert.True(t, strings.HasPrefix(preemptibleUnsupportedGauge.WithLabelValues("_", "mcm").Desc().String(),
		"Desc{fqName: \"d8_openstack_preemptible_unsupported\""),
		"the alert PromQL selects this metric name — rename requires an alert update")
	// Cleanup the probe pair so it doesn't pollute other tests.
	DeletePreemptibleUnsupported("_")
}
