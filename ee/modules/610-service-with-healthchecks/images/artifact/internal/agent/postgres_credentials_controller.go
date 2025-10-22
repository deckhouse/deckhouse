/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Currently, the code uses a fasthttp probes implementation, but this implementation,
// which uses only the standard library, was left as a backup.

package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	secretTypePostgresqlCredentials = "network.deckhouse.io/postgresql-credentials"
)

type PostgreSQLCredentialsReconciler struct {
	client.Client
	Logger logr.Logger
	Scheme *runtime.Scheme

	secretsCache sync.Map
}

func (r *PostgreSQLCredentialsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger.V(1).Info("reconcile secret", "name", req.Name, "namespace", req.Namespace)

	var creds PostgreSQLCredentials

	var secret corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &secret); err != nil {
		r.Logger.V(0).Error(err, "unable to fetch Secret", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	if secret.DeletionTimestamp != nil {
		r.secretsCache.Delete(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	creds.TlsMode = getNativeTLSMode(string(secret.Data["tlsMode"]))
	creds.User = string(secret.Data["user"])
	creds.Password = string(secret.Data["password"])
	creds.ClientCert = string(secret.Data["clientCert"])
	creds.ClientKey = string(secret.Data["clientKey"])
	creds.CaCert = string(secret.Data["caCert"])

	r.secretsCache.Store(req.NamespacedName, creds)

	return ctrl.Result{}, nil
}

func (r *PostgreSQLCredentialsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldSecret := e.ObjectOld.(*corev1.Secret)
				newSecret := e.ObjectNew.(*corev1.Secret)

				if newSecret.Type != secretTypePostgresqlCredentials {
					return false
				}
				return oldSecret.ResourceVersion != newSecret.ResourceVersion
			},
			CreateFunc: func(e event.CreateEvent) bool {
				secret := e.Object.(*corev1.Secret)
				if secret.Type != secretTypePostgresqlCredentials {
					return false
				}
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				secret := e.Object.(*corev1.Secret)
				if secret.Type != secretTypePostgresqlCredentials {
					return false
				}
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		Complete(r)
}

func (r *PostgreSQLCredentialsReconciler) findObjectsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret := obj.(*corev1.Secret)
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		},
	}
}

func (r *PostgreSQLCredentialsReconciler) GetCachedSecret(key types.NamespacedName) (PostgreSQLCredentials, error) {
	value, exists := r.secretsCache.Load(key)
	if !exists {
		return PostgreSQLCredentials{}, fmt.Errorf("secret not found")
	}
	return value.(PostgreSQLCredentials), nil
}
