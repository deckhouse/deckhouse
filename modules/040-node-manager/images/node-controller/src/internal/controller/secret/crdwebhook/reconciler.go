/*
Copyright 2025 Flant JSC

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

package crdwebhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

// watchedSecrets is the set of Secret names this reconciler reacts to.
var watchedSecrets = map[string]struct{}{
	"capi-webhook-tls":                    {},
	"caps-controller-manager-webhook-tls": {},
	"node-controller-webhook-tls":         {},
}

func init() {
	dynr.RegisterReconciler(rcname.CRDWebhook, &corev1.Secret{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler watches webhook TLS secrets in the d8-cloud-instance-manager
// namespace and patches CA bundles into the corresponding CRD conversion
// webhook configurations.
//
// It unifies the logic from three hooks:
//   - capi_crds_cabundle_injection (capi-webhook-tls → CAPI CRDs)
//   - sshcredentials_crd_cabundle_injection (caps-controller-manager-webhook-tls → sshcredentials CRD)
//   - nodegroup_crd_conversion_webhook (node-controller-webhook-tls → nodegroups CRD)
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{secretNamePredicate()}
}

// secretNamePredicate filters events to only secrets in the watched set
// within the d8-cloud-instance-manager namespace.
func secretNamePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isWatchedSecret(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isWatchedSecret(e.ObjectNew)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

func isWatchedSecret(obj client.Object) bool {
	if obj.GetNamespace() != webhookNamespace {
		return false
	}
	_, ok := watchedSecrets[obj.GetName()]
	return ok
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Only process secrets in the expected namespace.
	if req.Namespace != webhookNamespace {
		return ctrl.Result{}, nil
	}

	// Only process known webhook TLS secrets.
	if _, ok := watchedSecrets[req.Name]; !ok {
		return ctrl.Result{}, nil
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("secret not found, skipping", "secret", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get secret %s/%s: %w", req.Namespace, req.Name, err)
	}

	caBundle, ok := secret.Data["ca.crt"]
	if !ok || len(caBundle) == 0 {
		log.V(1).Info("secret has no ca.crt data, skipping", "secret", req.Name)
		return ctrl.Result{}, nil
	}

	if err := patchCRDsCABundle(ctx, r.Client, log, req.Name, caBundle); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch CRDs for secret %s: %w", req.Name, err)
	}

	return ctrl.Result{}, nil
}
