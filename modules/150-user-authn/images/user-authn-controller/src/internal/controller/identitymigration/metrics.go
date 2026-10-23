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

package identitymigration

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	pendingMetricName = "d8_user_authn_identity_migration_pending"
	totalMetricName   = "d8_user_authn_identity_migration_total"

	// resultMigrated: the UserIdentity was created or changed.
	resultMigrated = "migrated"
	// resultUnchanged: it already held everything, or the old places hold nothing.
	resultUnchanged = "unchanged"
	// resultSkipped: the pair cannot be copied (see the skip reasons); it is not retried.
	resultSkipped = "skipped"
	// resultFailed: an API error; the pair is retried.
	resultFailed = "failed"
)

var (
	registerMetricsOnce sync.Once

	pendingGauge = newPendingGauge()
	totalCounter = newTotalCounter()
)

func newPendingGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: pendingMetricName,
		Help: "Provider and user pairs whose data in the old Dex storage places (OfflineSessions, Password) " +
			"is not copied into UserIdentity yet; at least 1 while the first pass is running. Set by the leader only.",
	})
}

func newTotalCounter() *prometheus.CounterVec {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: totalMetricName,
		Help: "Copies of a provider and user pair into UserIdentity, by result: migrated, unchanged, skipped, failed.",
	}, []string{"result"})
	for _, result := range []string{resultMigrated, resultUnchanged, resultSkipped, resultFailed} {
		c.WithLabelValues(result)
	}
	return c
}

func registerMetrics() error {
	var regErr error
	registerMetricsOnce.Do(func() {
		for _, collector := range []prometheus.Collector{pendingGauge, totalCounter} {
			if err := ctrlmetrics.Registry.Register(collector); err != nil {
				var already prometheus.AlreadyRegisteredError
				if errors.As(err, &already) {
					continue
				}
				regErr = fmt.Errorf("register identity migration metrics: %w", err)
				return
			}
		}
	})
	return regErr
}

// progress tracks the pairs that still have to be copied and publishes their number.
type progress struct {
	mu          sync.Mutex
	pending     map[string]struct{}
	passRunning bool
	gauge       prometheus.Gauge
}

func newProgress(gauge prometheus.Gauge) *progress {
	return &progress{pending: map[string]struct{}{}, gauge: gauge}
}

// startPass marks the first pass as running: until it ends, how many pairs remain is unknown.
func (p *progress) startPass() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.passRunning = true
	p.publish()
}

// finishPass marks every pair as enqueued.
func (p *progress) finishPass() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.passRunning = false
	p.publish()
}

// add records a pair that is enqueued or failed.
func (p *progress) add(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending[name] = struct{}{}
	p.publish()
}

// done records a pair that needs nothing more.
func (p *progress) done(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pending[name]; !ok {
		return
	}
	delete(p.pending, name)
	p.publish()
}

// value is the number published, for tests.
func (p *progress) value() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count()
}

func (p *progress) count() int {
	n := len(p.pending)
	if p.passRunning {
		n++
	}
	return n
}

func (p *progress) publish() {
	p.gauge.Set(float64(p.count()))
}
