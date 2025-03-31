/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &servicesController{}

type servicesController struct {
	Namespace string
	NodeName  string
	Client    client.Client
	Services  *servicesManager
}

func (sc *servicesController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	controllerName := "services-controller"

	configRequest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      sc.getConfigSecretName(),
			Namespace: sc.Namespace,
		},
	}

	configPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		secret, ok := obj.(*corev1.Secret)

		if !ok {
			return false
		}

		return secret.Namespace == configRequest.Namespace && secret.Name == configRequest.Name
	})

	newConfigHandler := func(objectType string) handler.EventHandler {
		return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			log := ctrl.LoggerFrom(ctx)

			log.Info(
				"Reconcile will be triggered by object change",
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
				"type", objectType,
				"controller", controllerName,
			)

			return []reconcile.Request{configRequest}
		})
	}

	nodePredicate := predicate.Funcs{
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)

			if node.Name != sc.NodeName {
				return false
			}

			return hasMasterLabel(node)
		},
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)

			if node.Name != sc.NodeName {
				return false
			}

			return hasMasterLabel(node)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)

			if node.Name != sc.NodeName {
				return false
			}

			return hasMasterLabel(node)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldNode := e.ObjectNew.(*corev1.Node)
			newNode := e.ObjectNew.(*corev1.Node)

			if oldNode.Name != sc.NodeName || newNode.Name != sc.NodeName {
				return false
			}

			return hasMasterLabel(oldNode) != hasMasterLabel(newNode)
		},
	}

	moduleConfig := getModuleConfigObject()
	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == moduleConfig.GetName()
	})

	err := ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(
			&corev1.Secret{},
			builder.WithPredicates(configPredicate),
		).
		Watches(
			&moduleConfig,
			newConfigHandler("ModuleConfig"),
			builder.WithPredicates(moduleConfigPredicate),
		).
		Watches(
			&corev1.Node{},
			newConfigHandler("Node"),
			builder.WithPredicates(nodePredicate),
		).
		Complete(sc)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil

}

func (sc *servicesController) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	node := corev1.Node{}
	key := types.NamespacedName{Name: sc.NodeName}
	if err := sc.Client.Get(ctx, key, &node); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get node: %w", err)
	}

	if !hasMasterLabel(&node) {
		log.Info("Node is not master, stopping services")
		if changes, err := sc.Services.StopServices(); err == nil {
			log.Info("All servies stopped", "changes", changes)
			return reconcile.Result{}, nil
		} else {
			err = fmt.Errorf("cannot stop services: %w", err)
			log.Error(err, "Cannot stop services", "changes", changes)
			return reconcile.Result{}, err
		}
	}

	moduleConfig := getModuleConfigObject()
	key = types.NamespacedName{Name: moduleConfig.GetName()}
	if err := sc.Client.Get(ctx, key, &moduleConfig); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get ModuleConfig: %w", err)
	}

	moduleEnabled := true
	if enabled, found, _ := unstructured.NestedBool(moduleConfig.Object, "spec", "enabled"); found {
		moduleEnabled = enabled
	}
	log = log.WithValues("module_enabled", moduleEnabled)

	config := corev1.Secret{}
	key = types.NamespacedName{Name: sc.getConfigSecretName(), Namespace: sc.Namespace}
	if err := sc.Client.Get(ctx, key, &config); apierrors.IsNotFound(err) {
		log.Info("Config not found, should stop all!")
		return reconcile.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get Config: %w", err)
	}

	configVersion := string(config.Data["version"])
	configYAML := string(config.Data["config"])

	log.Info("Reconcile",
		"version", configVersion,
		"config_len", len(configYAML),
	)

	return ctrl.Result{}, nil
}

func (sc *servicesController) getConfigSecretName() string {
	return fmt.Sprintf("registry-node-config-%s", sc.NodeName)
}

func hasMasterLabel(node *corev1.Node) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	return isMaster
}

func getModuleConfigObject() unstructured.Unstructured {
	ret := unstructured.Unstructured{}
	ret.SetAPIVersion(moduleConfigApiVersion)
	ret.SetKind(moduleConfigKind)
	ret.SetName(registryModuleName)

	return ret
}
