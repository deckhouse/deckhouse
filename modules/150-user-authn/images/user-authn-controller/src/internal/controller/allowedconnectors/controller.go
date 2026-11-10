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

// Package allowedconnectors deletes the refresh tokens that a client no longer allows.
//
// A DexClient or a DexAuthenticator limits its providers with allowedProviders, which ends up as allowedConnectors of
// its OAuth2Client. Dex refuses a refresh token issued through a connector outside the list, but it keeps the token.
// Allow the provider again and the token works again, though its user may have lost the right to sign in while it was
// refused. So the controller deletes the tokens of the client that were issued through a connector outside the list,
// as soon as the list is set or changes. It keeps no state: a token outside the list is refused already, so deleting
// it again changes nothing for anybody.
package allowedconnectors

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
)

// Revoker deletes the refresh tokens of a client issued through connectors outside a list.
type Revoker interface {
	DeleteClientTokensOutside(ctx context.Context, clientID string, allowed []string) (int, error)
}

var _ Revoker = (*revoke.Revoker)(nil)

// Reconciler watches the OAuth2Client objects of Dex.
type Reconciler struct {
	// reader is the cached client of the manager: the controller has no right to get OAuth2Clients from the API
	// server, and the cache keeps the id and allowedConnectors of a client (revocationfeed.TransformClient).
	reader  client.Reader
	revoker Revoker
}

// New returns a Reconciler.
func New(reader client.Reader, revoker Revoker) *Reconciler {
	return &Reconciler{reader: reader, revoker: revoker}
}

// clients are what the reconciler takes from the manager.
type clients interface {
	GetClient() client.Client
	GetAPIReader() client.Reader
}

// newReconciler reads the OAuth2Clients through the cached client of the manager, never through its API reader: the
// controller may list and watch OAuth2Clients, not get them. The API reader goes to the revoker, for the objects it
// has the right to.
func newReconciler(m clients, revoker Revoker) *Reconciler {
	return New(m.GetClient(), revoker)
}

// Register wires the reconciler onto the manager.
//
// Required RBAC: oauth2clients list, watch (the cache); the rights of the revoke package for the refresh tokens and the
// OfflineSessions.
func Register(mgr manager.Manager) error {
	log := ctrl.Log.WithName("allowedconnectors")
	r := newReconciler(mgr, revoke.New(mgr.GetClient(), mgr.GetAPIReader(), log, nil))
	return ctrl.NewControllerManagedBy(mgr).
		Named("allowedconnectors").
		Watches(controller.Object(controller.OAuth2ClientGVK), &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// Reconcile deletes the tokens of the client the request names that its allowedConnectors refuse.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	obj := controller.Object(controller.OAuth2ClientGVK)
	if err := r.reader.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if obj.GetDeletionTimestamp() != nil {
		return reconcile.Result{}, nil
	}

	id, _, _ := unstructured.NestedString(obj.Object, "id")
	allowed, _, _ := unstructured.NestedStringSlice(obj.Object, "allowedConnectors")
	if id == "" || len(allowed) == 0 {
		return reconcile.Result{}, nil
	}
	if _, err := r.revoker.DeleteClientTokensOutside(ctx, id, allowed); err != nil {
		return reconcile.Result{}, fmt.Errorf("delete the refresh tokens of the client %s outside its allowedConnectors: %w", id, err)
	}
	return reconcile.Result{}, nil
}
