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

package namespace

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

// The namespaces waiting for adoption are reported with their creation time, the adopted, system
// and deleting ones are not, and a namespace adopted since drops out of the report.
func TestNotAdoptedReport(t *testing.T) {
	created := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	namespace := func(name string, labels map[string]string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, CreationTimestamp: metav1.NewTime(created)}}
	}
	r := newReconciler(t,
		namespace("waiting", nil),
		namespace("adopted", map[string]string{v1alpha3.ResourceLabelProject: "adopted"}),
		namespace("d8-system", nil),
		namespace("upmeter-probe", nil),
	)
	t.Cleanup(notAdopted.Reset)
	reporter := newNotAdoptedReporter(r.client, logr.Discard())
	require.NoError(t, reporter.report(context.Background()))
	assert.Equal(t, 1, testutil.CollectAndCount(notAdopted))
	assert.InDelta(t, float64(created.Unix()), testutil.ToFloat64(notAdopted.WithLabelValues("waiting")), 0)

	waiting := new(corev1.Namespace)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "waiting"}, waiting))
	waiting.Labels = map[string]string{v1alpha3.ResourceLabelProject: "waiting"}
	require.NoError(t, r.client.Update(context.Background(), waiting))
	require.NoError(t, reporter.report(context.Background()))
	assert.Zero(t, testutil.CollectAndCount(notAdopted))
	assert.False(t, reporter.NeedLeaderElection(), "reported while no replica leads")
}

// recordingGauge keeps the series the reporter sets and, after every write, the names that have one,
// so that a test sees what a scrape between two writes would see.
type recordingGauge struct {
	series    map[string]float64
	snapshots []map[string]float64
}

func (g *recordingGauge) Set(namespace string, value float64) {
	g.series[namespace] = value
	g.snapshot()
}

func (g *recordingGauge) Delete(namespace string) {
	delete(g.series, namespace)
	g.snapshot()
}

func (g *recordingGauge) snapshot() { g.snapshots = append(g.snapshots, maps.Clone(g.series)) }

// A namespace that stays in the report keeps its series through the whole pass. The old pass emptied
// the vector and filled it again, so a scrape in between saw no series: Prometheus marked it stale and
// the pending alert started its wait over.
func TestNotAdoptedReportNeverDropsASeriesThatStays(t *testing.T) {
	created := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	namespace := func(name string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)}}
	}
	r := newReconciler(t, namespace("stays"), namespace("leaves"))
	gauge := &recordingGauge{series: map[string]float64{}}
	reporter := newNotAdoptedReporter(r.client, logr.Discard())
	reporter.gauge = gauge
	ctx := context.Background()

	require.NoError(t, reporter.report(ctx))
	assert.Equal(t, map[string]float64{"stays": float64(created.Unix()), "leaves": float64(created.Unix())}, gauge.series)

	leaves := new(corev1.Namespace)
	require.NoError(t, r.client.Get(ctx, client.ObjectKey{Name: "leaves"}, leaves))
	leaves.Labels = map[string]string{v1alpha3.ResourceLabelProject: "leaves"}
	require.NoError(t, r.client.Update(ctx, leaves))
	require.NoError(t, r.client.Create(ctx, namespace("arrives")))

	first := len(gauge.snapshots)
	require.NoError(t, reporter.report(ctx))
	require.Greater(t, len(gauge.snapshots), first, "the second pass changes the series")
	for _, snapshot := range gauge.snapshots[first:] {
		assert.Contains(t, snapshot, "stays", "a series that stays is there between every two writes")
	}
	assert.Equal(t, map[string]float64{"stays": float64(created.Unix()), "arrives": float64(created.Unix())}, gauge.series)

	require.NoError(t, reporter.report(ctx))
	assert.Equal(t, map[string]float64{"stays": float64(created.Unix()), "arrives": float64(created.Unix())}, gauge.series)
}

// The metric names the namespace in a label of its own: Prometheus sets namespace, pod and the other
// target labels of the scrape to those of the controller pod, renames a metric label that collides
// with one to exported_<name>, and the alert then names d8-multitenancy-manager instead.
func TestNotAdoptedLabelsDoNotCollideWithTargetLabels(t *testing.T) {
	desc := make(chan *prometheus.Desc, 1)
	notAdopted.Describe(desc)
	description := (<-desc).String()
	assert.Contains(t, description, "variableLabels: {target_namespace}")
	for _, target := range []string{"namespace", "pod", "container", "service", "endpoint", "job", "instance"} {
		assert.NotContains(t, description, "{"+target+"}")
	}
}
