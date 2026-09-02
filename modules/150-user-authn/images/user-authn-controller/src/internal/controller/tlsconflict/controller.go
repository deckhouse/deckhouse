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
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

const (
	metricName = "d8_dex_provider_ldap_tls_conflict"

	conflictInsecureNoSSLStartTLS          = "insecureNoSSL+startTLS"
	conflictInsecureNoSSLInsecureSkipVerif = "insecureNoSSL+insecureSkipVerify"
	conflictInsecureNoSSLRootCAData        = "insecureNoSSL+rootCAData"
)

var (
	registerMetricsOnce sync.Once

	ldapTLSConflict = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricName,
			Help: "DexProvider LDAP TLS flag conflict (1 = conflicting insecureNoSSL combination).",
		},
		[]string{"name", "conflict"},
	)
)

// Reconciler exposes LDAP TLS conflict gauges for DexProvider objects.
type Reconciler struct {
	client client.Client
	gauge  *prometheus.GaugeVec
	mu     sync.Mutex
}

// New constructs a Reconciler. gauge defaults to the process-wide metric.
func New(c client.Client, gauge *prometheus.GaugeVec) *Reconciler {
	if gauge == nil {
		gauge = ldapTLSConflict
	}
	return &Reconciler{client: c, gauge: gauge}
}

// Register wires the TLS-conflict reconciler onto the manager and registers
// d8_dex_provider_ldap_tls_conflict on the controller-runtime registry.
//
// Required RBAC: dexproviders get, list, watch.
func Register(mgr manager.Manager) error {
	if err := registerMetrics(); err != nil {
		return err
	}
	r := New(mgr.GetClient(), ldapTLSConflict)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup tlsconflict controller: %w", err)
	}
	return nil
}

func registerMetrics() error {
	var regErr error
	registerMetricsOnce.Do(func() {
		if err := ctrlmetrics.Registry.Register(ldapTLSConflict); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) {
				return
			}
			regErr = fmt.Errorf("register %s: %w", metricName, err)
		}
	})
	return regErr
}

// SetupWithManager watches DexProvider objects.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	obj := controller.Object(controller.DexProviderGVK)
	return ctrl.NewControllerManagedBy(mgr).
		Named("tlsconflict").
		Watches(obj, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// Reconcile resets and sets conflict gauges from the current DexProvider list.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	list := controller.List(controller.DexProviderGVK)
	if err := r.client.List(ctx, list); err != nil {
		return reconcile.Result{}, fmt.Errorf("list dexproviders: %w", err)
	}

	samples := make([]conflictSample, 0, len(list.Items))
	for i := range list.Items {
		if err := ctx.Err(); err != nil {
			return reconcile.Result{}, err
		}
		found, err := conflictsOf(&list.Items[i])
		if err != nil {
			return reconcile.Result{}, err
		}
		samples = append(samples, found...)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauge.Reset()
	for _, sample := range samples {
		r.gauge.WithLabelValues(sample.name, sample.conflict).Set(1)
	}
	return reconcile.Result{}, nil
}

type conflictSample struct {
	name     string
	conflict string
}

func conflictsOf(obj *unstructured.Unstructured) ([]conflictSample, error) {
	name := obj.GetName()
	providerType, _, err := unstructured.NestedString(obj.Object, "spec", "type")
	if err != nil {
		return nil, fmt.Errorf("read spec.type from DexProvider %q: %w", name, err)
	}
	if providerType != "LDAP" {
		return nil, nil
	}

	insecureNoSSL, _, err := unstructured.NestedBool(obj.Object, "spec", "ldap", "insecureNoSSL")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.insecureNoSSL from DexProvider %q: %w", name, err)
	}
	if !insecureNoSSL {
		return nil, nil
	}

	startTLS, _, err := unstructured.NestedBool(obj.Object, "spec", "ldap", "startTLS")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.startTLS from DexProvider %q: %w", name, err)
	}
	insecureSkipVerify, _, err := unstructured.NestedBool(obj.Object, "spec", "ldap", "insecureSkipVerify")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.insecureSkipVerify from DexProvider %q: %w", name, err)
	}
	rootCA, _, err := unstructured.NestedString(obj.Object, "spec", "ldap", "rootCAData")
	if err != nil {
		return nil, fmt.Errorf("read spec.ldap.rootCAData from DexProvider %q: %w", name, err)
	}

	out := make([]conflictSample, 0, 3)
	if startTLS {
		out = append(out, conflictSample{name: name, conflict: conflictInsecureNoSSLStartTLS})
	}
	if insecureSkipVerify {
		out = append(out, conflictSample{name: name, conflict: conflictInsecureNoSSLInsecureSkipVerif})
	}
	if rootCA != "" {
		out = append(out, conflictSample{name: name, conflict: conflictInsecureNoSSLRootCAData})
	}
	return out, nil
}
