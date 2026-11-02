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

package ledger

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// The drift a leader found is not exported by it once it left the lease: the alert would name the
// former leader, and the runbook would restart the wrong pod.
func TestStepDownClearsTheDrift(t *testing.T) {
	clock := time.Now()
	r := &runner{
		log: logr.Discard(), metrics: NewMetrics(prometheus.NewRegistry()), webhook: &http.Server{ReadHeaderTimeout: time.Second},
		lock: &FencedLock{Interface: &slowLock{clock: &clock}, Now: func() time.Time { return clock }},
	}
	r.leader.Store(true)
	r.metrics.Drift.Set(3)

	r.stepDown()

	assert.Zero(t, testutil.ToFloat64(r.metrics.Drift))
}
