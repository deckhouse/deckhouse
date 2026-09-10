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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	monitoringManifestKey = "monitoring.yaml.tpl"

	// monitorWatcherLabelKey gates whether Deckhouse Prometheus looks at monitors in a namespace at
	// all (podMonitorNamespaceSelector). Without it they are ignored silently - no error, no event.
	//
	// Setting it is the user's job, documented alongside every other PodMonitor in Deckhouse; modules
	// only ever put it on namespaces their own chart creates. This one belongs to whoever created the
	// VirtualControlPlane and may well be reconciled by Argo CD or Flux, which would strip the label
	// straight back and leave the application permanently OutOfSync. So: observe and report, never patch.
	monitorWatcherLabelKey = "prometheus.deckhouse.io/monitor-watcher-enabled"
)

var monitorGVKs = []schema.GroupVersionKind{
	{Group: "monitoring.coreos.com", Version: "v1", Kind: "PodMonitor"},
	{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"},
}

// reconcileMonitoring publishes the per-VCP monitors. A no-op when the monitor CRDs are absent
// (operator-prometheus disabled): a missing Prometheus must not fail the reconcile.
func (r *reconciler) reconcileMonitoring(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	available, err := r.monitorKindsAvailable()
	if err != nil {
		return reconcile.Result{}, err
	}
	if !available {
		// The controller deliberately does not Own() these kinds: a watch on an absent CRD fails at
		// manager start, and operator-prometheus is optional. The requeue interval picks them up.
		log.FromContext(ctx).V(1).Info("monitoring CRDs are absent, skipping VCP monitors")
		return reconcile.Result{}, nil
	}

	if err := r.reconcileMetricsToken(ctx, vcp); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.warnOnMissingMonitorWatcherLabel(ctx, vcp.Namespace); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.applyParentManifests(ctx, vcp, configSecret, monitoringManifestKey)
}

func (r *reconciler) monitorKindsAvailable() (bool, error) {
	for _, gvk := range monitorGVKs {
		_, err := r.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("resolve %s: %w", gvk.Kind, err)
		}
	}
	return true, nil
}

// warnOnMissingMonitorWatcherLabel turns a silent misconfiguration into a log line naming the fix:
// without the label Prometheus ignores the monitors and nothing anywhere reports why.
func (r *reconciler) warnOnMissingMonitorWatcherLabel(ctx context.Context, namespace string) error {
	// Namespace is on the client's cache DisableFor list, so this is a live read: one extra GET per
	// VCP per requeue interval, and no informer on a cluster-scoped kind.
	ns := &corev1.Namespace{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		return fmt.Errorf("get namespace %s: %w", namespace, err)
	}
	if ns.Labels[monitorWatcherLabelKey] == "true" {
		return nil
	}

	log.FromContext(ctx).Info(
		"VirtualControlPlane monitors will be ignored by Prometheus: namespace is missing the watcher label",
		"namespace", namespace,
		"label", monitorWatcherLabelKey+"=true",
	)
	return nil
}
