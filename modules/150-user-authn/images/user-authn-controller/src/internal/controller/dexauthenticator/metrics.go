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

package dexauthenticator

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/naming"
)

const (
	// The telemetry of the DexAuthenticator objects, which the module hook exported while the chart
	// rendered them: the names and the labels are the ones the fleet statistics already collect.
	telemetryObjectsMetric      = "d8_telemetry_user_authn_dex_authenticators"
	telemetryApplicationsMetric = "d8_telemetry_user_authn_dex_authenticator_applications"

	tableBytesMetric = "d8_user_authn_shared_dex_authenticator_table_bytes"
	tableLimitMetric = "d8_user_authn_shared_dex_authenticator_table_limit_bytes"
	notServedMetric  = "d8_user_authn_shared_dex_authenticator_objects_not_served"
	// objectNotServedMetric has a series per object whose Served condition is not True, and none
	// for an object that is served. The alerts D8DexAuthenticatorNotServed and
	// D8DexAuthenticatorWaitingForInstance name the object by it:
	//
	//	max by (dex_authenticator_namespace, dex_authenticator_name, instance_name, reason) (d8_user_authn_dex_authenticator_not_served) > 0
	objectNotServedMetric = "d8_user_authn_dex_authenticator_not_served"

	objectNamespaceLabel = "dex_authenticator_namespace"
	objectNameLabel      = "dex_authenticator_name"

	originModule = "module"
	originUser   = "user"
)

// Metrics are the metrics of the table reconciler and of the reconciler of the objects.
type Metrics struct {
	objects         *prometheus.GaugeVec
	applications    *prometheus.GaugeVec
	tableBytes      *prometheus.GaugeVec
	tableLimit      prometheus.Gauge
	notServed       *prometheus.GaugeVec
	objectNotServed *prometheus.GaugeVec
}

// NewMetrics registers the metrics in registry.
func NewMetrics(registry prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		objects: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: telemetryObjectsMetric,
			Help: "DexAuthenticator objects by who created them and what they use. Set by the leader only.",
		}, []string{"origin", "high_availability", "sign_out", "kubernetes_access"}),
		applications: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: telemetryApplicationsMetric,
			Help: "Applications (domains) of the DexAuthenticator objects by who created the object. Set by the leader only.",
		}, []string{"origin"}),
		tableBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: tableBytesMetric,
			Help: "Size of the table of clients of a shared dex-authenticator instance, in bytes. Set by the leader only.",
		}, []string{"instance_name"}),
		tableLimit: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: tableLimitMetric,
			Help: "The largest table of clients a shared dex-authenticator instance can get, in bytes: " +
				"a DexAuthenticator object that does not fit is not served.",
		}),
		notServed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: notServedMetric,
			Help: "DexAuthenticator objects that have no record in the table of their instance or are not served on all " +
				"their domains, by the reason of the Served condition; TableFull that comes from the share of the namespace " +
				"of the object is " + reasonNamespaceShareFull + ". Set by the leader only.",
		}, []string{"instance_name", "reason"}),
		objectNotServed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: objectNotServedMetric,
			Help: "DexAuthenticator objects whose Served condition is not True, by the reason of the condition: 1 for " +
				"such an object, no series for an object that is served; TableFull that comes from the share of the namespace " +
				"of the object is " + reasonNamespaceShareFull + ". Set by the leader only.",
		}, []string{objectNamespaceLabel, objectNameLabel, "instance_name", "reason"}),
	}
	for _, collector := range []prometheus.Collector{m.objects, m.applications, m.tableBytes, m.tableLimit, m.notServed, m.objectNotServed} {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// observe sets the metrics from the result of a pass of the table reconciler.
func (m *Metrics) observe(authenticators []Authenticator, result buildResult, limit int) {
	if m == nil {
		return
	}

	type objectKey struct{ origin, highAvailability, signOut, kubernetesAccess string }
	objects := make(map[objectKey]float64)
	applications := map[string]float64{originModule: 0, originUser: 0}
	for _, a := range authenticators {
		origin := originUser
		if a.Module {
			origin = originModule
		}
		highAvailability := "unset"
		if a.HighAvailability != nil {
			highAvailability = strconv.FormatBool(*a.HighAvailability)
		}
		signOut := false
		for _, app := range a.Applications {
			signOut = signOut || app.SignOutURL != ""
		}
		objects[objectKey{origin, highAvailability, strconv.FormatBool(signOut), strconv.FormatBool(a.AllowAccessToKubernetes)}]++
		applications[origin] += float64(len(a.Applications))
	}
	m.objects.Reset()
	for key, count := range objects {
		m.objects.WithLabelValues(key.origin, key.highAvailability, key.signOut, key.kubernetesAccess).Set(count)
	}
	for origin, count := range applications {
		m.applications.WithLabelValues(origin).Set(count)
	}

	m.tableLimit.Set(float64(limit))
	for instance, built := range result.tables {
		m.tableBytes.WithLabelValues(instance).Set(float64(built.size))
	}

	type reasonKey struct{ instance, reason string }
	notServed := make(map[reasonKey]float64)
	for _, entry := range result.entries {
		if entry.Reason != "" {
			notServed[reasonKey{entry.Instance, metricReason(entry)}]++
		}
	}
	m.notServed.Reset()
	for key, count := range notServed {
		m.notServed.WithLabelValues(key.instance, key.reason).Set(count)
	}
}

// reasonNamespaceShareFull is the reason a metric gives the reason TableFull of an object that
// did not fit into the share of its namespace (NamespaceShare): the table itself has room, and
// the alert on a full table must not fire for it.
const reasonNamespaceShareFull = "NamespaceShareFull"

// metricReason is the reason the metrics give the object of entry.
func metricReason(entry Entry) string {
	if entry.Reason == ReasonTableFull && entry.OverNamespaceShare {
		return reasonNamespaceShareFull
	}
	return entry.Reason
}

// notServedBecause records that the object key is not served for reason: the object has one
// series, with the reason it has now.
func (m *Metrics) notServedBecause(key types.NamespacedName, reason string) {
	if m == nil {
		return
	}
	m.served(key)
	m.objectNotServed.WithLabelValues(key.Namespace, key.Name, naming.AuthenticatorInstance(key.Namespace), reason).Set(1)
}

// served records that the object key is served or is gone: it has no series.
func (m *Metrics) served(key types.NamespacedName) {
	if m == nil {
		return
	}
	m.objectNotServed.DeletePartialMatch(prometheus.Labels{objectNamespaceLabel: key.Namespace, objectNameLabel: key.Name})
}
