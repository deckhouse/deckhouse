/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	labelTypeKey             = "type"
	labelNodeSecretTypeValue = "node-secret"
	labelHeritageKey         = "heritage"
	labelHeritageValue       = "deckhouse"
)

type NodeReconciler = nodeReconciler

var _ reconcile.Reconciler = &nodeReconciler{}

type nodeReconciler struct {
	client client.Client
	log    logr.Logger
}

func (r *nodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	r.log = mgr.GetLogger().
		WithName("Node-Reconciler").
		WithValues("component", "NodeReconciler")

	r.client = mgr.GetClient()

	err := ctrl.NewControllerManagedBy(mgr).
		Named("node-controller").
		For(&corev1.Node{}).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(nodePkiSecretMapFunc),
			builder.WithPredicates(predicate.NewPredicateFuncs(nodePkiSecretPredicate)),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (r *nodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("Reconcile Start",
		"name", req.Name,
		"namespace", req.Namespace,
		"namespaced-name", req.NamespacedName,
	)
	defer r.log.Info("Reconcile Done",
		"name", req.Name,
		"namespace", req.Namespace,
		"namespaced-name", req.NamespacedName,
	)

	return ctrl.Result{}, nil
}

func nodePkiSecretMapFunc(ctx context.Context, o client.Object) []reconcile.Request {
	var ret reconcile.Request

	name := o.GetName()
	sub := nodePKISecretRegex.FindStringSubmatch(name)

	if len(sub) < 2 {
		return nil
	}

	ret.Namespace = fmt.Sprintf("-SECRET-%v-", name)
	ret.Name = sub[1]

	return []reconcile.Request{ret}
}

func nodePkiSecretPredicate(o client.Object) bool {
	labels := o.GetLabels()

	if labels[labelTypeKey] != labelNodeSecretTypeValue {
		return false
	}

	if labels[labelHeritageKey] != labelHeritageValue {
		return false
	}

	return nodePKISecretRegex.MatchString(o.GetName())
}
