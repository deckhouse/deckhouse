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

package kuberbacauth

import (
	"crypto/x509"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/deckhouse/deckhouse/pkg/kube-rbac-auth/internal/metrics"
)

// counters returns the series of a counter, each as its "label=value" pairs in name order, with their values.
// A counter that New did not register has its name as HELP, and fails the test.
func counters(t *testing.T, storage *metricsstorage.MetricStorage, metric string) map[string]float64 {
	t.Helper()
	families, err := storage.Gather()
	if err != nil {
		t.Fatal(err)
	}
	series := make(map[string]float64)
	for _, family := range families {
		if family.GetName() != metric {
			continue
		}
		if family.GetHelp() == metric {
			t.Fatalf("%s is not registered: its HELP is its name", metric)
		}
		for _, m := range family.GetMetric() {
			labels := make([]string, 0, len(m.GetLabel()))
			for _, label := range m.GetLabel() {
				labels = append(labels, label.GetName()+"="+label.GetValue())
			}
			slices.Sort(labels)
			series[strings.Join(labels, ",")] = m.GetCounter().GetValue()
		}
	}
	return series
}

func expectCounters(t *testing.T, storage *metricsstorage.MetricStorage, metric string, want map[string]float64) {
	t.Helper()
	if got := counters(t, storage, metric); !maps.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", metric, got, want)
	}
}

// Every check of a request is counted with its result and the credential it is about, and every request
// to the API server with its result. New registers the counters, so the labels counted must be the registered ones.
func TestRequestMetrics(t *testing.T) {
	ca := newTestCA(t, "kubernetes")
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	writeCAFile(t, caFile, ca)
	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	e := newEnv(t, Config{ClientCAFile: caFile, MetricStorage: storage})
	stranger := newTestCA(t, "another-ca").issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth)

	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", ""), http.StatusUnauthorized)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer wrong"), http.StatusUnauthorized)
	// A certificate that is not trusted is no credential.
	expectCode(t, e.doCert(t, stranger, ""), http.StatusUnauthorized)
	expectCode(t, e.doCert(t, ca.issue(t, "kubernetes-admin", nil, x509.ExtKeyUsageClientAuth), ""), http.StatusOK)
	e.api.set(func(f *fakeAPI) { f.allowed = false })
	expectCode(t, e.doCert(t, ca.issue(t, "jane", nil, x509.ExtKeyUsageClientAuth), ""), http.StatusForbidden)
	// Every step of the backoff is a request to the API server.
	e.api.set(func(f *fakeAPI) { f.down = true })
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer other"), http.StatusInternalServerError)
	// Excluded paths and paths without a rule are not checked.
	expectCode(t, e.do(t, http.MethodGet, "/other", "Bearer "+validToken), http.StatusNotFound)

	expectCounters(t, storage, metrics.RequestsTotal, map[string]float64{
		"credential=token,result=allowed":         1,
		"credential=none,result=unauthorized":     2,
		"credential=token,result=unauthorized":    1,
		"credential=certificate,result=allowed":   1,
		"credential=certificate,result=forbidden": 1,
		"credential=token,result=error":           1,
	})
	expectCounters(t, storage, metrics.ReviewRequestsTotal, map[string]float64{
		"result=success,review=TokenReview":         2,
		"result=error,review=TokenReview":           3,
		"result=success,review=SubjectAccessReview": 3,
	})
	expectCounters(t, storage, metrics.StaleResultsTotal, map[string]float64{})
}

// A result used after its lifetime because the API server could not answer is counted by review.
func TestStaleResultMetrics(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	e := newEnv(t, Config{StaleCacheTTL: time.Hour, MetricStorage: storage})
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)

	e.api.set(func(f *fakeAPI) { f.down = true })
	e.clock.step(30 * time.Minute)
	expectCode(t, e.do(t, http.MethodGet, "/metrics", "Bearer "+validToken), http.StatusOK)

	expectCounters(t, storage, metrics.StaleResultsTotal, map[string]float64{
		"review=TokenReview":         1,
		"review=SubjectAccessReview": 1,
	})
	expectCounters(t, storage, metrics.ReviewRequestsTotal, map[string]float64{
		"result=success,review=TokenReview":         1,
		"result=error,review=TokenReview":           3,
		"result=success,review=SubjectAccessReview": 1,
		"result=error,review=SubjectAccessReview":   3,
	})
	expectCounters(t, storage, metrics.RequestsTotal, map[string]float64{"credential=token,result=allowed": 2})
}

// The middlewares of one application share its storage. A metric of another type under the same name
// makes New fail instead of losing the counts.
func TestMetricStorageRegistration(t *testing.T) {
	rules := []Rule{{Path: "/metrics", ResourceAttributes: metricsAttrs}}
	api := &fakeAPI{}

	shared := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	for range 2 {
		if _, err := New(api.client(), Config{Rules: rules, MetricStorage: shared}); err != nil {
			t.Fatalf("New with a shared storage: %v", err)
		}
	}

	taken := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	if _, err := taken.RegisterGauge(metrics.RequestsTotal, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := New(api.client(), Config{Rules: rules, MetricStorage: taken}); err == nil || !strings.HasPrefix(err.Error(), "register metrics: ") {
		t.Fatalf("err = %v, want a registration error", err)
	}
}
