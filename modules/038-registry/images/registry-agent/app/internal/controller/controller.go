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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	healthz "sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"registry-agent/internal/config"
	"registry-agent/internal/containerd"
	"registry-agent/internal/proxy"
)

// RouterHolder holds a *proxy.Router that can be swapped atomically.
type RouterHolder struct {
	ptr atomic.Pointer[proxy.Router]
}

// Get returns the current router (may be nil before first reconcile).
func (h *RouterHolder) Get() *proxy.Router {
	return h.ptr.Load()
}

// Set atomically replaces the router.
func (h *RouterHolder) Set(r *proxy.Router) {
	h.ptr.Store(r)
}

// Reconciler reconciles a RegistryConfig custom resource.
type Reconciler struct {
	Client      client.Client
	RegistryDir string
	Opts        config.Options
	Routers     *RouterHolder
	// Ready is flipped to true after the first successful reconcile (containerd
	// configured + marker written + routes live). Wired to /readyz so the DaemonSet
	// reports a node ready only once its agent owns registry.d. Optional (may be nil).
	Ready *atomic.Bool
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconcile fetches the RegistryConfig CR, builds the containerd desired state
// and proxy routes, and applies them. It updates the CR status on every
// reconcile to reflect success or the last error.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "RegistryConfig",
	})
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cfg, err := parseRegistryConfig(obj)
	if err != nil {
		log.Error(err, "failed to parse RegistryConfig")
		if statusErr := r.setStatus(ctx, obj, false, err.Error(), 0); statusErr != nil {
			log.Error(statusErr, "failed to update status after parse error")
		}
		return ctrl.Result{}, err
	}

	ds, routes, err := config.Build(cfg, r.Opts)
	if err != nil {
		log.Error(err, "failed to build desired state")
		if statusErr := r.setStatus(ctx, obj, false, err.Error(), 0); statusErr != nil {
			log.Error(statusErr, "failed to update status after build error")
		}
		return ctrl.Result{}, err
	}

	if err := containerd.Reconcile(r.RegistryDir, ds); err != nil {
		log.Error(err, "failed to reconcile containerd config")
		return ctrl.Result{}, err
	}

	r.Routers.Set(proxy.NewRouter(dedupRoutes(routes)))

	if r.Ready != nil {
		r.Ready.Store(true)
	}

	if err := r.setStatus(ctx, obj, true, "", obj.GetGeneration()); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// statusUnchanged returns true when the desired status fields already match what
// is stored in obj.Object["status"], so we can skip the Status().Update call
// and avoid triggering a watch event that would re-enqueue the CR.
func statusUnchanged(obj *unstructured.Unstructured, ready bool, message string, observedGeneration int64) bool {
	curReady, _, _ := unstructured.NestedBool(obj.Object, "status", "ready")
	curGen, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
	curMsg, _, _ := unstructured.NestedString(obj.Object, "status", "message")
	return curReady == ready && curGen == observedGeneration && curMsg == message
}

// setStatus writes the given ready/message/observedGeneration into the CR status.
// It is a no-op when the stored status already equals the desired values, which
// prevents the controller's own write from re-enqueuing the CR.
func (r *Reconciler) setStatus(ctx context.Context, obj *unstructured.Unstructured, ready bool, message string, observedGeneration int64) error {
	if statusUnchanged(obj, ready, message, observedGeneration) {
		return nil
	}
	status := map[string]interface{}{
		"ready":              ready,
		"observedGeneration": observedGeneration,
	}
	if message != "" {
		status["message"] = message
	}
	if err := unstructured.SetNestedField(obj.Object, status, "status"); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

// SetupWithManager registers the Reconciler with the controller-runtime manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "RegistryConfig",
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("registry-config").
		For(u, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// ReadyzCheck returns a controller-runtime health checker that reports ready
// only once the agent has completed its first successful reconcile.
func ReadyzCheck(ready *atomic.Bool) healthz.Checker {
	return func(_ *http.Request) error {
		if ready.Load() {
			return nil
		}
		return errors.New("agent has not completed its first reconcile")
	}
}

// dedupRoutes returns a copy of routes with duplicates by (NS, PathPrefix)
// removed (last-wins). The original order of surviving entries is preserved.
// Deduping by NS alone would collapse module-source routes — which all share
// NS=PrimaryHost and are distinguished only by PathPrefix — into one (and drop
// the primary's own default route), so the (NS, PathPrefix) pair is the key.
func dedupRoutes(routes []proxy.Route) []proxy.Route {
	seen := make(map[string]struct{}, len(routes))
	out := make([]proxy.Route, 0, len(routes))
	for i := len(routes) - 1; i >= 0; i-- {
		key := routes[i].NS + "\x00" + routes[i].PathPrefix
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, routes[i])
	}
	// Reverse to restore original order of surviving entries.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
