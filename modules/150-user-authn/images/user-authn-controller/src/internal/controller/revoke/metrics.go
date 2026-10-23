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

package revoke

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	storedMetricName  = "d8_user_authn_revocations"
	writtenMetricName = "d8_user_authn_revocations_written_total"
)

var (
	registerMetricsOnce sync.Once

	storedGauge    = newStoredGauge()
	writtenCounter = newWrittenCounter()
)

// newStoredGauge returns the gauge of stored revocations without series: only the leader counts
// them (GC), and a replica that is not the leader must not export zeros next to its count.
func newStoredGauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: storedMetricName,
		Help: "Revocation objects kept in the Dex namespace, by type: pair, session, grant. Exported by the leader " +
			"only, after every garbage collection pass.",
	}, []string{"type"})
}

func newWrittenCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: writtenMetricName,
		Help: "Revocation objects the controller created, by type and reason.",
	}, []string{"type", "reason"})
}

// RegisterMetrics registers the metrics of the package in the registry of controller-runtime.
func RegisterMetrics() error {
	var regErr error
	registerMetricsOnce.Do(func() {
		for _, collector := range []prometheus.Collector{storedGauge, writtenCounter} {
			if err := ctrlmetrics.Registry.Register(collector); err != nil {
				var already prometheus.AlreadyRegisteredError
				if errors.As(err, &already) {
					continue
				}
				regErr = fmt.Errorf("register revocation metrics: %w", err)
				return
			}
		}
	})
	return regErr
}
