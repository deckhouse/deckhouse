/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"embeded-registry-manager/internal/utils/k8s"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	RegistryModuleName    = "system-registry"
	RegistryROSecretName  = "registry-user-ro"
	RegistryRwSecretName  = "registry-user-rw"
	RegistryPKISecretName = "registry-pki"
)

type StateController = stateController

var _ reconcile.Reconciler = &stateController{}

type stateController struct {
	Client            client.Client
	Namespace         string
	ReprocessAllNodes func(ctx context.Context) error
}

func (sc *stateController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if sc.ReprocessAllNodes == nil {
		return fmt.Errorf("please set ReprocessAllNodes field")
	}

	controllerName := "global-state-controller"

	moduleConfig := &unstructured.Unstructured{}
	moduleConfig.SetAPIVersion(k8s.ModuleConfigApiVersion)
	moduleConfig.SetKind(k8s.ModuleConfigKind)

	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == RegistryModuleName
	})

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != sc.Namespace {
			return false
		}

		name := obj.GetName()
		return name == RegistryPKISecretName || name == RegistryROSecretName || name == RegistryRwSecretName
	})

	secretsHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx)

		log.Info(
			"Secret was changed, will trigger reconcile",
			"secret", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"controller", controllerName,
		)

		var req reconcile.Request
		req.Name = RegistryModuleName

		return []reconcile.Request{req}
	})

	err := ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(moduleConfig, builder.WithPredicates(moduleConfigPredicate)).
		Watches(
			&corev1.Secret{},
			secretsHandler,
			builder.WithPredicates(secretsPredicate),
		).
		Complete(sc)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (sc *stateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("--Reconcile--")

	err := sc.ReprocessAllNodes(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot reprocess all nodes: %w", err)
	}

	return ctrl.Result{}, nil
}
