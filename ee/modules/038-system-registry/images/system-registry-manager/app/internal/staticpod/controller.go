/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
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
	Namespace    string
	NodeName     string
	PodName      string
	PodNamespace string

	Client   client.Client
	Services *servicesManager
}

func (sc *servicesController) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
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
		GenericFunc: func(_ event.TypedGenericEvent[client.Object]) bool {
			return false
		},
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			return false
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)
			return node.Name == sc.NodeName
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldNode := e.ObjectOld.(*corev1.Node)
			newNode := e.ObjectNew.(*corev1.Node)

			if oldNode.Name != sc.NodeName || newNode.Name != sc.NodeName {
				return false
			}

			return hasMasterLabel(newNode) != hasMasterLabel(oldNode)
		},
	}

	moduleConfig := getModuleConfigObject()
	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == moduleConfig.GetName()
	})

	podPredicate := predicate.Funcs{
		GenericFunc: func(_ event.TypedGenericEvent[client.Object]) bool {
			return false
		},
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			return false
		},
		DeleteFunc: func(_ event.TypedDeleteEvent[client.Object]) bool {
			return false
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldPod := e.ObjectOld.(*corev1.Pod)
			newPod := e.ObjectNew.(*corev1.Pod)

			if oldPod.Name != sc.PodName || oldPod.Namespace != sc.PodNamespace {
				return false
			}

			if newPod.Name != sc.PodName || newPod.Namespace != sc.PodNamespace {
				return false
			}

			return isPodReady(newPod) != isPodReady(oldPod)
		},
	}

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
		Watches(
			&corev1.Pod{},
			newConfigHandler("Pod"),
			builder.WithPredicates(podPredicate),
		).
		Complete(sc)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (sc *servicesController) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	isMaster, err := sc.checkNode(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot check node: %w", err)
	}

	if !isMaster {
		log.Info("Our Node is not master, stopping services")

		err = sc.stopServices(ctx)
		if err != nil {
			err = fmt.Errorf("cannot stop services: %w", err)
		}
		return ctrl.Result{}, err
	}

	moduleEnabled, err := sc.checkModuleEnabled(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot check module enabled: %w", err)
	}

	if !moduleEnabled {
		log.Info("Our Module is not enabled, stopping services")

		err := sc.stopServices(ctx)
		if err != nil {
			err = fmt.Errorf("cannot stop services: %w", err)
		}
		return ctrl.Result{}, err
	}

	if !sc.checkPodReady(ctx) {
		log.Info("Our pod is not ready, skipping reconcile")
		return ctrl.Result{}, nil
	}

	if err := sc.processConfig(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot process config: %w", err)
	}

	return ctrl.Result{}, nil
}

func (sc *servicesController) checkNode(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	var node corev1.Node
	key := types.NamespacedName{Name: sc.NodeName}

	if err := sc.Client.Get(ctx, key, &node); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// How?
			log.Error(err, "Node is not found")
			return false, nil
		}
		return false, fmt.Errorf("cannot get node: %w", err)
	}

	return hasMasterLabel(&node), nil
}

func (sc *servicesController) checkModuleEnabled(ctx context.Context) (bool, error) {
	moduleConfig := getModuleConfigObject()
	key := types.NamespacedName{Name: moduleConfig.GetName()}

	if err := sc.Client.Get(ctx, key, &moduleConfig); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("cannot get ModuleConfig: %w", err)
	}

	enabled, _, _ := unstructured.NestedBool(moduleConfig.Object, "spec", "enabled")
	return enabled, nil
}

func (sc *servicesController) checkPodReady(ctx context.Context) bool {
	log := ctrl.LoggerFrom(ctx)

	var pod corev1.Pod
	key := types.NamespacedName{Namespace: sc.PodNamespace, Name: sc.PodName}

	if err := sc.Client.Get(ctx, key, &pod); err != nil {
		log.Error(err, "Cannot get our pod")
		return false
	}

	return isPodReady(&pod)
}

func (sc *servicesController) processConfig(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	config := corev1.Secret{}
	key := types.NamespacedName{Name: sc.getConfigSecretName(), Namespace: sc.Namespace}

	if err := sc.Client.Get(ctx, key, &config); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return sc.stopServices(ctx)
		}

		return fmt.Errorf("cannot get Config: %w", err)
	}

	configModel := NodeServicesConfigModel{
		Version: string(config.Data["version"]),
	}

	if err := yaml.Unmarshal(config.Data["config"], &configModel.Config); err != nil {
		return fmt.Errorf("config unmarshal error: %w", err)
	}

	if err := configModel.Validate(); err != nil {
		return fmt.Errorf("config validation error: %w", err)
	}

	changes, err := sc.Services.applyConfig(configModel)
	if err != nil {
		return fmt.Errorf("apply services config error: %w", err)
	}

	if changes.HasChanges() {
		log.Info(
			"Services configuration created/updated successfully",
			"changes", changes,
			"version", configModel.Version,
		)
	} else {
		log.Info("No changes in services configuration required")
	}

	return nil
}

func (sc *servicesController) stopServices(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	changes, err := sc.Services.StopServices()

	if err != nil {
		return fmt.Errorf("stop services error: %w", err)
	}

	if changes.HasChanges() {
		log.Info(
			"All services are stopped successfully",
			"changes", changes,
		)
	} else {
		log.Info("All services are stopped already")
	}

	return nil
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
	ret.SetAPIVersion(moduleConfigAPIVersion)
	ret.SetKind(moduleConfigKind)
	ret.SetName(registryModuleName)

	return ret
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == "Ready" {
			return cond.Status == "True"
		}
	}

	return false
}
