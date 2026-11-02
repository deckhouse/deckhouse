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
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// notAdoptedEvery is how often the namespaces waiting for adoption are reported.
const notAdoptedEvery = 30 * time.Second

var notAdopted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "d8_multitenancy_namespace_not_adopted",
	Help: "The creation time, in seconds since the epoch, of a namespace that belongs to no project and that the controller is to adopt into a project of its own. " +
		"Every replica exports it, the one that does not lead too, so it is seen while no replica leads.",
}, []string{"target_namespace"})

func init() {
	metrics.Registry.MustRegister(notAdopted)
}

// seriesGauge is the part of the gauge the reporter updates: one series per namespace.
type seriesGauge interface {
	Set(namespace string, value float64)
	Delete(namespace string)
}

// vecGauge is the series of the not-adopted metric.
type vecGauge struct{ vec *prometheus.GaugeVec }

func (g vecGauge) Set(namespace string, value float64) { g.vec.WithLabelValues(namespace).Set(value) }

func (g vecGauge) Delete(namespace string) { g.vec.DeleteLabelValues(namespace) }

// notAdoptedReporter exports the namespaces the controller is to adopt and has not adopted yet. A
// namespace is adopted seconds after it appears, so one that stays is a sign that the controller does
// not lead or fails to adopt it; an object created in it meanwhile skips the checks of the project. It
// runs on every replica, as the leader may be the one that is missing.
type notAdoptedReporter struct {
	reader client.Reader
	logger logr.Logger
	gauge  seriesGauge
	// reported holds the namespaces that have a series now. Only report touches it, and it runs on one
	// goroutine.
	reported map[string]struct{}
}

func newNotAdoptedReporter(reader client.Reader, logger logr.Logger) *notAdoptedReporter {
	return &notAdoptedReporter{
		reader:   reader,
		logger:   logger,
		gauge:    vecGauge{vec: notAdopted},
		reported: map[string]struct{}{},
	}
}

// NeedLeaderElection is false: the report is to be there while no replica leads.
func (r *notAdoptedReporter) NeedLeaderElection() bool { return false }

func (r *notAdoptedReporter) Start(ctx context.Context) error {
	ticker := time.NewTicker(notAdoptedEvery)
	defer ticker.Stop()
	for {
		if err := r.report(ctx); err != nil {
			r.logger.Error(err, "report the namespaces not adopted")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// report brings the series to the namespaces waiting for adoption now. A series is removed only when
// its namespace leaves the set, and the others are set in place: the vector is never emptied and
// filled again, so a scrape in between cannot miss a series that stays. A series that vanishes from a
// scrape is marked stale in Prometheus, and the pending alert on it starts its wait over.
func (r *notAdoptedReporter) report(ctx context.Context) error {
	list := new(corev1.NamespaceList)
	if err := r.reader.List(ctx, list); err != nil {
		return err
	}
	waiting := make(map[string]float64, len(r.reported))
	for i := range list.Items {
		ns := &list.Items[i]
		if !ns.DeletionTimestamp.IsZero() || !isAdoptionCandidate(ns) {
			continue
		}
		waiting[ns.Name] = float64(ns.CreationTimestamp.Unix())
	}
	for name := range r.reported {
		if _, stays := waiting[name]; !stays {
			r.gauge.Delete(name)
			delete(r.reported, name)
		}
	}
	for name, created := range waiting {
		r.gauge.Set(name, created)
		r.reported[name] = struct{}{}
	}
	return nil
}
