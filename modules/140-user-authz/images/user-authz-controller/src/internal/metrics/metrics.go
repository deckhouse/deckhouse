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

// Package metrics exposes the state of the reconcilers as Prometheus metrics.
//
// The reconcilers report per-object observations (how many bindings a rule wants, how many it
// has, how far apart the two are); the Collector keeps the latest observation of every object and
// aggregates them per kind when scraped, so that the number of series does not grow with the
// number of rules. Only invalid rules are exported one series each, because the alert has to name
// the object.
//
// All metrics are prefixed d8_user_authz_. The reconcile duration and error counters come from
// controller-runtime (controller_runtime_reconcile_*).
package metrics

import (
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "d8_user_authz"

// Kinds of reconciled objects, the value of the kind label.
const (
	KindClusterAuthorizationRule = "ClusterAuthorizationRule"
	KindAuthorizationRule        = "AuthorizationRule"
	KindDict                     = "dict"
	KindManage                   = "manage"
)

// Operations and results of the apply counter.
const (
	OpCreate = "create"
	OpUpdate = "update"
	OpDelete = "delete"

	ResultSuccess = "success"
	ResultError   = "error"
)

// Drift reasons.
const (
	DriftMissing = "missing"
	DriftExtra   = "extra"
	DriftChanged = "changed"
)

// MaxInvalidSeries bounds the number of invalid rules exported one series each. A cluster-wide
// write failure marks every rule invalid at once; the aggregate per kind and reason is always
// complete, the named series are for drilling down.
const MaxInvalidSeries = 50

// Drift is how far the bindings of one object are from the desired state, by reason.
type Drift struct {
	Missing int // desired but absent
	Extra   int // present but not desired
	Changed int // present but differing
}

// Total is the number of bindings that are not in the desired state.
func (d Drift) Total() int { return d.Missing + d.Extra + d.Changed }

// Observation is the latest state of one reconciled object.
type Observation struct {
	Desired int
	Actual  int
	Drift   Drift
}

type invalidRule struct {
	kind, name, namespace, reason string
}

// Collector aggregates the observations of the reconcilers. It implements prometheus.Collector.
type Collector struct {
	mu       sync.Mutex
	observed map[string]map[string]Observation // kind -> object key -> observation
	invalid  map[string]invalidRule            // kind + object key -> labels

	applyTotal *prometheus.CounterVec

	descDesired      *prometheus.Desc
	descActual       *prometheus.Desc
	descDrift        *prometheus.Desc
	descRules        *prometheus.Desc
	descInvalid      *prometheus.Desc
	descInvalidTotal *prometheus.Desc
}

// Default is the process-wide collector the reconcilers use.
var Default = New()

// New constructs an empty Collector.
func New() *Collector {
	return &Collector{
		observed: make(map[string]map[string]Observation),
		invalid:  make(map[string]invalidRule),
		applyTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bindings_apply_total",
			Help:      "Write operations on bindings issued by the controller, by kind of the reconciled object, operation and result.",
		}, []string{"kind", "op", "result"}),
		descDesired: prometheus.NewDesc(namespace+"_bindings_desired",
			"Bindings the reconciled objects of a kind must have.", []string{"kind"}, nil),
		descActual: prometheus.NewDesc(namespace+"_bindings_actual",
			"Bindings of the reconciled objects of a kind after their last reconcile; equals desired once the controller converged.", []string{"kind"}, nil),
		descDrift: prometheus.NewDesc(namespace+"_bindings_drift",
			"Bindings still not in the desired state after the last reconcile, by reason (missing, extra, changed); above zero only while the controller fails to converge.", []string{"kind", "reason"}, nil),
		descRules: prometheus.NewDesc(namespace+"_authorization_rules",
			"Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).", []string{"kind"}, nil),
		descInvalid: prometheus.NewDesc(namespace+"_authorization_rule_invalid",
			"Rules whose bindings cannot be applied (1 per rule, at most MaxInvalidSeries rules), with the reason of the Ready=False condition.", []string{"kind", "name", "rule_namespace", "reason"}, nil),
		descInvalidTotal: prometheus.NewDesc(namespace+"_authorization_rules_invalid",
			"Number of rules whose bindings cannot be applied, by kind and reason of the Ready=False condition.", []string{"kind", "reason"}, nil),
	}
}

// Register registers the collector and its counters on reg. Registering twice is not an error.
func (c *Collector) Register(reg prometheus.Registerer) error {
	for _, collector := range []prometheus.Collector{c, c.applyTotal} {
		if err := reg.Register(collector); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) {
				continue
			}
			return err
		}
	}
	return nil
}

// Observe records the latest state of the object key of the given kind.
func (c *Collector) Observe(kind, key string, o Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	byKey, ok := c.observed[kind]
	if !ok {
		byKey = make(map[string]Observation)
		c.observed[kind] = byKey
	}
	byKey[key] = o
}

// Forget drops every series of the object key of the given kind (the object is gone).
func (c *Collector) Forget(kind, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.observed[kind], key)
	delete(c.invalid, kind+"/"+key)
}

// SetInvalid marks the rule as not applicable for the given reason (the Ready=False reason).
func (c *Collector) SetInvalid(kind, key, name, namespace, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalid[kind+"/"+key] = invalidRule{kind: kind, name: name, namespace: namespace, reason: reason}
}

// ClearInvalid removes the invalid mark of the rule.
func (c *Collector) ClearInvalid(kind, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.invalid, kind+"/"+key)
}

// ApplyTotal exposes the write counter (for tests and dashboards built in code).
func (c *Collector) ApplyTotal() *prometheus.CounterVec { return c.applyTotal }

// RecordApply counts one write operation.
func (c *Collector) RecordApply(kind, op string, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	c.applyTotal.WithLabelValues(kind, op, result).Inc()
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descDesired
	ch <- c.descActual
	ch <- c.descDrift
	ch <- c.descRules
	ch <- c.descInvalid
	ch <- c.descInvalidTotal
}

// Collect implements prometheus.Collector: the observations are summed per kind.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for kind, byKey := range c.observed {
		var desired, actual int
		var drift Drift
		for _, o := range byKey {
			desired += o.Desired
			actual += o.Actual
			drift.Missing += o.Drift.Missing
			drift.Extra += o.Drift.Extra
			drift.Changed += o.Drift.Changed
		}
		ch <- prometheus.MustNewConstMetric(c.descRules, prometheus.GaugeValue, float64(len(byKey)), kind)
		ch <- prometheus.MustNewConstMetric(c.descDesired, prometheus.GaugeValue, float64(desired), kind)
		ch <- prometheus.MustNewConstMetric(c.descActual, prometheus.GaugeValue, float64(actual), kind)
		ch <- prometheus.MustNewConstMetric(c.descDrift, prometheus.GaugeValue, float64(drift.Missing), kind, DriftMissing)
		ch <- prometheus.MustNewConstMetric(c.descDrift, prometheus.GaugeValue, float64(drift.Extra), kind, DriftExtra)
		ch <- prometheus.MustNewConstMetric(c.descDrift, prometheus.GaugeValue, float64(drift.Changed), kind, DriftChanged)
	}

	type kindReason struct{ kind, reason string }
	totals := make(map[kindReason]int)
	for i, key := range slices.Sorted(maps.Keys(c.invalid)) {
		rule := c.invalid[key]
		totals[kindReason{rule.kind, rule.reason}]++
		if i < MaxInvalidSeries {
			ch <- prometheus.MustNewConstMetric(c.descInvalid, prometheus.GaugeValue, 1, rule.kind, rule.name, rule.namespace, rule.reason)
		}
	}
	for kr, n := range totals {
		ch <- prometheus.MustNewConstMetric(c.descInvalidTotal, prometheus.GaugeValue, float64(n), kr.kind, kr.reason)
	}
}
