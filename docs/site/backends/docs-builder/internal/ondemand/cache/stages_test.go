// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/metrics"
)

func TestStoreReportsStages(t *testing.T) {
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if err := metrics.RegisterMetrics(ms); err != nil {
		t.Fatal(err)
	}
	cold := newMemCold()
	f := &fakeRender{size: 10}
	s := newColdStore(t, f, cold)
	s.cfg.Metrics = ms
	renderDone(t, s, "echo", "v1.0.0")
	cold.waitFor(t, "echo/v1.0.0/rev1.tar.gz")

	restarted := newColdStore(t, f, cold)
	restarted.cfg.Metrics = ms
	renderDone(t, restarted, "echo", "v1.0.0")

	rec := httptest.NewRecorder()
	ms.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, series := range []string{
		`docs_builder_ondemand_stage_duration_seconds_count{image="",stage="publish"} 2`,
		`docs_builder_ondemand_stage_duration_seconds_count{image="cold",stage="download"} 1`,
	} {
		if !strings.Contains(rec.Body.String(), series) {
			t.Errorf("no %s", series)
		}
	}
}
