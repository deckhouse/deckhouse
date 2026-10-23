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
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/revocation"
)

func revocationObj(t *testing.T, name, typ string, created time.Time, serverTime bool) *unstructured.Unstructured {
	t.Helper()
	obj, err := revocation.Revocation{Name: name, Type: typ, ConnectorID: "local", UserID: "jane", CreatedAt: created}.Object()
	if err != nil {
		t.Fatal(err)
	}
	if serverTime {
		obj.SetCreationTimestamp(metav1.NewTime(created))
	}
	return obj
}

// Not parallel: it reads the gauge of the package.
func TestGC_DeletesWhatIsPastRetention(t *testing.T) {
	retention := 70 * time.Minute
	undecodable := revocationObj(t, "pair-broken-g1", revocation.TypePair, revokeTime.Add(-time.Minute), true)
	undecodable.Object["generation"] = "three"
	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		revocationObj(t, "pair-old-g1", revocation.TypePair, revokeTime.Add(-retention-time.Second), true),
		revocationObj(t, "pair-fresh-g2", revocation.TypePair, revokeTime.Add(-retention+time.Second), true),
		revocationObj(t, "grant-old", revocation.TypeGrant, revokeTime.Add(-2*time.Hour), false),
		revocationObj(t, "session-fresh", revocation.TypeSession, revokeTime.Add(-time.Minute), false),
		undecodable,
	)
	gc := NewGC(c, retention, logr.Discard(), func() time.Time { return revokeTime })
	if err := gc.Collect(t.Context()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	for name, kept := range map[string]bool{
		"pair-old-g1":    false,
		"grant-old":      false,
		"pair-fresh-g2":  true,
		"session-fresh":  true,
		"pair-broken-g1": true,
	} {
		if got := exists(t, c, controller.RevocationGVK, name); got != kept {
			t.Errorf("%s kept = %v, want %v", name, got, kept)
		}
	}
	for typ, expected := range map[string]float64{revocation.TypePair: 2, revocation.TypeSession: 1, revocation.TypeGrant: 0} {
		if got := metricValue(t, storedGauge.WithLabelValues(typ)); got != expected {
			t.Errorf("stored %s = %v, want %v", typ, got, expected)
		}
	}
}

// seriesOf counts the series a collector exports.
func seriesOf(c prometheus.Collector) int {
	ch := make(chan prometheus.Metric, 16)
	c.Collect(ch)
	close(ch)
	n := 0
	for range ch {
		n++
	}
	return n
}

// m12: a replica that is not the leader never ran GC and exports no stored revocations, rather than
// zeros next to the count of the leader.
func TestStoredGauge_HasNoSeriesUntilCounted(t *testing.T) {
	t.Parallel()

	if n := seriesOf(newStoredGauge()); n != 0 {
		t.Errorf("series = %d before a garbage collection pass, want 0", n)
	}
}

// Not parallel: it reads the gauge of the package. The leader exports a series per type while it
// runs GC; once GC stops - the replica stops leading - the series go.
func TestGC_StartExportsTheGaugeWhileLeading(t *testing.T) {
	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		revocationObj(t, "pair-fresh-g1", revocation.TypePair, revokeTime.Add(-time.Minute), true),
	)
	gc := NewGC(c, time.Hour, logr.Discard(), func() time.Time { return revokeTime })
	storedGauge.Reset() // what an earlier test counted
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- gc.Start(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for seriesOf(storedGauge) != 3 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("series = %d while leading, want 3", seriesOf(storedGauge))
		}
		time.Sleep(time.Millisecond)
	}
	if got := metricValue(t, storedGauge.WithLabelValues(revocation.TypePair)); got != 1 {
		t.Errorf("stored pair = %v, want 1", got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after the context ended")
	}
	if n := seriesOf(storedGauge); n != 0 {
		t.Errorf("series = %d after GC stopped, want 0", n)
	}
}

func TestGC_StartStopsWithTheContext(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{})
	gc := NewGC(c, time.Hour, logr.Discard(), nil)
	if !gc.NeedLeaderElection() {
		t.Error("GC must run on the leader only")
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- gc.Start(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after the context ended")
	}
}
