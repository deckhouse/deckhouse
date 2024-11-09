/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

func (r *RegistryReconciler) secretEventHandler() handler.EventHandler {
	secretsToWatch := []string{
		"registry-user-ro",
		"registry-user-rw",
		"registry-pki",
	}

	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		secretName := obj.GetName()

		// Helper function to enqueue reconcile request
		enqueue := func(name, namespace string) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: client.ObjectKey{
					Name:      name,
					Namespace: namespace,
				}},
			}
		}

		// Check if the secret name matches the list
		for _, currentSecretName := range secretsToWatch {
			if secretName == currentSecretName {
				return enqueue(obj.GetName(), obj.GetNamespace())
			}
		}

		// Check for the "registry-node-*-pki" pattern
		if strings.HasPrefix(secretName, "registry-node-") && strings.HasSuffix(secretName, "-pki") {
			return enqueue(obj.GetName(), obj.GetNamespace())
		}

		return nil
	})
}

func (r *RegistryReconciler) handleModuleConfigCreate(ctx context.Context, obj interface{}) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("CREATE")
	r.handleModuleConfigChange(ctx, obj)
}

func (r *RegistryReconciler) handleModuleConfigChange(ctx context.Context, obj interface{}) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("CHANGE")
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		logger.Error(fmt.Errorf("failed to convert object to unstructured"), "failed to convert object to unstructured")
		return
	}

	enabled, settingsMap, err := r.extractModuleConfigFieldsFromObject(unstructuredObj)
	if err != nil {
		logger.Error(err, "failed to extract fields from ModuleConfig")
		return
	}

	delete(settingsMap, "imagesOverride")

	settingsJSON, err := json.Marshal(settingsMap)
	if err != nil {
		logger.Error(err, "failed to marshal settings map to JSON")
		return
	}

	var settings RegistryConfig
	err = json.Unmarshal(settingsJSON, &settings)
	if err != nil {
		logger.Error(err, "failed to unmarshal JSON to RegistryConfig struct")
		return
	}

	r.embeddedRegistry.Mutex.Lock()
	r.embeddedRegistry.mc.Enabled = enabled
	r.embeddedRegistry.mc.Settings = settings
	r.embeddedRegistry.Mutex.Unlock()

	logger.Info("ModuleConfig updated", "enabled", r.embeddedRegistry.mc.Enabled, "settings", r.embeddedRegistry.mc.Settings)
}

func (r *RegistryReconciler) handleModuleConfigDelete(ctx context.Context) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Error(fmt.Errorf("ModuleConfig was deleted"), "ModuleConfig was deleted")
	r.embeddedRegistry.Mutex.Lock()
	defer r.embeddedRegistry.Mutex.Unlock()
	// Reset the ModuleConfig settings to default when deleted
	r.embeddedRegistry.mc.Enabled = false
	r.embeddedRegistry.mc.Settings = RegistryConfig{}
}
