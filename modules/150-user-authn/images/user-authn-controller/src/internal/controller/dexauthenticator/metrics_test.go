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

package dexauthenticator

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/naming"
)

// TestMetrics_NotServedTellsTheShareFromAFullTable: an object refused for the share of its
// namespace is counted apart from one the table has no room for, so the alert on a full table does
// not fire while the table has room.
func TestMetrics_NotServedTellsTheShareFromAFullTable(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	instance := naming.UserAuthenticatorInstance
	metrics.observe(nil, buildResult{entries: map[types.NamespacedName]Entry{
		keyOf("greedy", "a"): {Instance: instance, Reason: ReasonTableFull, OverNamespaceShare: true},
		keyOf("greedy", "b"): {Instance: instance, Reason: ReasonTableFull, OverNamespaceShare: true},
		keyOf("shop", "c"):   {Instance: instance, Reason: ReasonTableFull},
	}}, TableLimit)

	value := func(reason string) float64 {
		t.Helper()
		m := &dto.Metric{}
		if err := metrics.notServed.WithLabelValues(instance, reason).Write(m); err != nil {
			t.Fatal(err)
		}
		return m.GetGauge().GetValue()
	}
	if got := value(ReasonTableFull); got != 1 {
		t.Errorf("TableFull = %v, want 1", got)
	}
	if got := value(reasonNamespaceShareFull); got != 2 {
		t.Errorf("%s = %v, want 2", reasonNamespaceShareFull, got)
	}
}
