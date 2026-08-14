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

package tlsconflict

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

func TestReconcile_ConflictCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		objects   []client.Object
		want      map[string]float64
		wantCount int
	}{
		{
			name:      "no providers",
			wantCount: 0,
		},
		{
			name: "non-LDAP ignored",
			objects: []client.Object{dexProvider("gitlab", map[string]any{
				"type": "OIDC",
				"oidc": map[string]any{"issuer": "https://gitlab.example.com", "clientID": "abc", "clientSecret": "xyz"},
			})},
			wantCount: 0,
		},
		{
			name: "LDAP without conflict",
			objects: []client.Object{dexProvider("ldap-ok", map[string]any{
				"type": "LDAP",
				"ldap": map[string]any{
					"host":               "ldap.example.org:389",
					"startTLS":           true,
					"insecureSkipVerify": true,
				},
			})},
			wantCount: 0,
		},
		{
			name: "insecureNoSSL+startTLS",
			objects: []client.Object{dexProvider("ldap-bad-starttls", map[string]any{
				"type": "LDAP",
				"ldap": map[string]any{
					"host":          "ldap.example.org:389",
					"insecureNoSSL": true,
					"startTLS":      true,
				},
			})},
			want: map[string]float64{
				"ldap-bad-starttls|" + conflictInsecureNoSSLStartTLS: 1,
			},
			wantCount: 1,
		},
		{
			name: "insecureNoSSL+insecureSkipVerify and rootCAData",
			objects: []client.Object{dexProvider("ldap-bad-multi", map[string]any{
				"type": "LDAP",
				"ldap": map[string]any{
					"host":               "ldap.example.org:389",
					"insecureNoSSL":      true,
					"insecureSkipVerify": true,
					"rootCAData":         "-----BEGIN CERTIFICATE-----\nMIIFaDC...\n-----END CERTIFICATE-----\n",
				},
			})},
			want: map[string]float64{
				"ldap-bad-multi|" + conflictInsecureNoSSLInsecureSkipVerif: 1,
				"ldap-bad-multi|" + conflictInsecureNoSSLRootCAData:        1,
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gauge := newTestGauge()
			r := newTestReconciler(t, gauge, tt.objects...)
			if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "any"}}); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			got := collectGauge(t, gauge)
			if len(got) != tt.wantCount {
				t.Fatalf("metric count = %d, want %d (%v)", len(got), tt.wantCount, got)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Errorf("metric %s = %v, want %v (all=%v)", key, got[key], want, got)
				}
			}
			for key := range got {
				if strings.Contains(key, "BEGIN CERTIFICATE") {
					t.Fatalf("cert material leaked into metric labels: %s", key)
				}
			}
		})
	}
}

func TestReconcile_ResolvedConflictClearsMetric(t *testing.T) {
	t.Parallel()

	gauge := newTestGauge()
	bad := dexProvider("ldap-flip", map[string]any{
		"type": "LDAP",
		"ldap": map[string]any{
			"host":          "ldap.example.org:389",
			"insecureNoSSL": true,
			"startTLS":      true,
		},
	})
	r := newTestReconciler(t, gauge, bad)
	if _, err := r.Reconcile(t.Context(), reconcile.Request{}); err != nil {
		t.Fatalf("first Reconcile: %v", err)
	}
	if got := collectGauge(t, gauge); len(got) != 1 {
		t.Fatalf("first run metric count = %d, want 1 (%v)", len(got), got)
	}

	fixed := dexProvider("ldap-flip", map[string]any{
		"type": "LDAP",
		"ldap": map[string]any{
			"host":          "ldap.example.org:389",
			"insecureNoSSL": true,
		},
	})
	r = newTestReconciler(t, gauge, fixed)
	if _, err := r.Reconcile(t.Context(), reconcile.Request{}); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if got := collectGauge(t, gauge); len(got) != 0 {
		t.Fatalf("resolved conflict still present: %v", got)
	}
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, newTestGauge())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, reconcile.Request{})
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func TestReconcile_CanceledContextDoesNotResetGauges(t *testing.T) {
	t.Parallel()

	gauge := newTestGauge()
	gauge.WithLabelValues("keep", conflictInsecureNoSSLStartTLS).Set(1)
	r := newTestReconciler(t, gauge, dexProvider("ldap-bad-starttls", map[string]any{
		"type": "LDAP",
		"ldap": map[string]any{
			"host":          "ldap.example.org:389",
			"insecureNoSSL": true,
			"startTLS":      true,
		},
	}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Reconcile(ctx, reconcile.Request{}); err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
	got := collectGauge(t, gauge)
	if got["keep|"+conflictInsecureNoSSLStartTLS] != 1 {
		t.Fatalf("canceled reconcile reset gauges: %v", got)
	}
}

func newTestGauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
		Help: "test",
	}, []string{"name", "conflict"})
}

func newTestReconciler(t *testing.T, gauge *prometheus.GaugeVec, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithObjects(objs...).
		Build()
	return New(c, gauge)
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1"}})
	m.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	return m
}

func dexProvider(name string, spec map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "DexProvider",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
	u.SetGroupVersionKind(controller.DexProviderGVK)
	return u
}

func collectGauge(t *testing.T, g *prometheus.GaugeVec) map[string]float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 16)
	g.Collect(ch)
	close(ch)
	out := map[string]float64{}
	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		var name, conflict string
		for _, lp := range pb.GetLabel() {
			switch lp.GetName() {
			case "name":
				name = lp.GetValue()
			case "conflict":
				conflict = lp.GetValue()
			}
		}
		out[name+"|"+conflict] = pb.GetGauge().GetValue()
	}
	return out
}
