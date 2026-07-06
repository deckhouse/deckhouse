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

package bashiblecontext

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("bashible-context", &v1.NodeGroup{}, &Controller{})
}

// Controller is the single writer of the bashible-apiserver-context Secret,
// replacing the helm define bashible_input_data. It reconciles on any NodeGroup
// change (primary For) and on changes to the kube objects the input.yaml blob is
// assembled from, re-running Assemble to rebuild the whole Secret each time.
//
// ⚠ It must never be enabled while the helm define still renders the same Secret
// — that would make two writers fight over one object (flapping). The cutover
// (enabling this + removing the helm define) is atomic and ships in one release.
type Controller struct {
	register.Base
}

// assembleRequest is the sentinel request the source-object watches enqueue.
// Reconcile ignores the request entirely and always rebuilds the whole Secret,
// so its identity is irrelevant — it only coalesces source events into a rebuild.
var assembleRequest = []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "assemble"}}}

// SetupWatches wires the source objects that feed the blob. NodeGroups are the
// primary watch (For); the rest are the kube objects ReadGlobals/Build/BuildElement
// read — Secrets (bootstrap tokens, certs, cloud provider, control-plane args,
// packages-proxy, cluster/static config), ConfigMaps (cluster-uuid, version-info),
// Services (DNS) and Pods (apiserver endpoints) — scoped to their namespaces so a
// change to any of them re-assembles the Secret.
func (c *Controller) SetupWatches(w register.Watcher) {
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return assembleRequest
	})
	w.Watches(&corev1.Secret{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS, cloudInstanceManagerNS)))
	w.Watches(&corev1.ConfigMap{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS, versionInfoCMNS)))
	w.Watches(&corev1.Service{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS)))
	w.Watches(&corev1.Pod{}, enqueue, builder.WithPredicates(inNamespaces(kubeSystemNS)))
}

// inNamespaces filters events to objects in the given namespaces, keeping the
// controller from waking on every Secret/ConfigMap in the cluster.
func inNamespaces(namespaces ...string) predicate.Predicate {
	set := make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		set[ns] = true
	}
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return set[obj.GetNamespace()]
	})
}

// Reconcile rebuilds the whole bashible-apiserver-context Secret from every
// NodeGroup and the source kube objects. The request is ignored: this is a
// singleton aggregator, so any trigger re-assembles the complete payload.
func (c *Controller) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	r := &Reconciler{
		Client:        c.Client,
		Context:       &Service{Client: c.Client},
		DerivedStatus: &derived_status.Service{Client: c.Client},
	}
	if err := r.Assemble(ctx); err != nil {
		logger.Error(err, "failed to assemble bashible-apiserver-context")
		return ctrl.Result{}, err
	}
	logger.V(1).Info("assembled bashible-apiserver-context")
	return ctrl.Result{}, nil
}
