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

// Package metrics is what this replica reports about its own work.
//
// Deliberately about the work and not about the state. Whether the cache is complete lives
// in RegistryStorage.status, where every replica's account of itself is collected and
// compared; duplicating it here would create a second answer to the same question, and the
// two would disagree exactly when it mattered. What is here instead is what only this
// process knows: how long a fill took, how many references it copied, how often it failed.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics are the collectors this replica updates.
type Metrics struct {
	// Leader is 1 while this replica holds the storage lease.
	//
	// Exported per replica rather than as one cluster-wide gauge, because the interesting
	// failures are disagreements: two replicas both believing they lead, or none of them
	// doing so. A single gauge could not express either.
	Leader prometheus.Gauge

	// FillDuration is how long a pass over the expected image set took.
	FillDuration prometheus.Histogram

	// References counts what a pass did with each reference it considered.
	References *prometheus.CounterVec

	// Passes counts completed passes by the action taken and whether it succeeded.
	Passes *prometheus.CounterVec

	// Collections counts garbage collections by outcome.
	Collections *prometheus.CounterVec

	// Reclaimed counts the references a collection removed.
	Reclaimed prometheus.Counter

	// LastCollection is when this replica last finished one, as a unix timestamp.
	//
	// A timestamp and not a counter, because the question is "how long has it been" — a
	// store that has not been reclaimed for a month is the thing worth alerting on, and no
	// rate over a nightly event can express it.
	//
	// Reads 0 until the first collection finishes, and that zero is not nothing: a registered
	// gauge is exposed from the first scrape, so anything computing `time() - this` gets the
	// whole of the Unix epoch and concludes the store was last reclaimed in 1970. Whoever reads
	// it has to know how long this replica has been able to collect, which is what Started is
	// for.
	LastCollection prometheus.Gauge

	// Started is when this replica's syncer began, as a unix timestamp.
	//
	// Exists so that "never collected" can be told from "collected long ago". Without it the
	// reclaim alert fired on every healthy new cluster: LastCollection reads 0, `time() - 0` is
	// past any threshold, and an alert with an hour's patience fires an hour after boot on a
	// cluster whose schedule had not yet come round once.
	Started prometheus.Gauge
}

// New registers the collectors and returns them.
func New(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		Leader: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_registry_storage_leader",
			Help: "1 while this replica holds the storage leader lease.",
		}),
		FillDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "d8_registry_storage_fill_duration_seconds",
			Help: "How long a pass over the expected image set took.",
			// Wide and coarse: filling a cache from an upstream is minutes to hours on a
			// slow link, and the useful question is which order of magnitude, not which
			// second.
			Buckets: []float64{1, 10, 60, 300, 900, 1800, 3600, 7200, 21600},
		}),
		References: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_storage_references_total",
			Help: "References considered by a fill or replication pass, by outcome.",
		}, []string{"outcome"}),
		Passes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_storage_passes_total",
			Help: "Completed passes, by the action taken and whether it succeeded.",
		}, []string{"action", "result"}),
	}

	m.Collections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_registry_storage_collections_total",
		Help: "Garbage collections of this replica's store, by outcome.",
	}, []string{"result"})
	m.Reclaimed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "d8_registry_storage_reclaimed_references_total",
		Help: "References removed by garbage collection.",
	})
	m.LastCollection = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_storage_last_collection_timestamp_seconds",
		Help: "When this replica last finished a garbage collection.",
	})
	m.Started = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_storage_started_timestamp_seconds",
		Help: "When this replica's syncer started.",
	})

	// Set here rather than by the caller: this runs once, at startup, and a start time nobody
	// remembered to set would read 0 — which would make the alert that depends on it fire on
	// every cluster instead of none, the same way the zero it exists to disambiguate did.
	m.Started.Set(float64(time.Now().Unix()))

	registry.MustRegister(m.Leader, m.FillDuration, m.References, m.Passes,
		m.Collections, m.Reclaimed, m.LastCollection, m.Started)
	return m
}

// Outcomes a reference can have in a pass.
const (
	OutcomeWritten = "written"
	OutcomeFailed  = "failed"
)

// Results a pass can have.
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)
