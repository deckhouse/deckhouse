/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
)

func (r *RegistryReconciler) handleNodeAdd(ctx context.Context, mgr ctrl.Manager, node *corev1.Node) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Node added", "nodeName", node.Name)

	r.embeddedRegistry.mutex.Lock()
	r.embeddedRegistry.masterNodes[node.Name] = k8s.MasterNode{
		Name:              node.Name,
		Address:           node.Status.Addresses[0].Address,
		CreationTimestamp: node.CreationTimestamp.Time,
	}
	r.embeddedRegistry.mutex.Unlock()

	select {
	case <-mgr.Elected():
		// Only the leader should reconcile
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      "registry-node-" + node.Name + "-pki",
				Namespace: k8s.RegistryNamespace,
			},
		}
		_, err := r.Reconcile(ctx, req)
		if err != nil {
			logger.Error(err, "Reconcile failed")
		}
	default:
		logger.Info("Not the leader, skipping Reconcile")
	}
}

func (r *RegistryReconciler) handleNodeDelete(ctx context.Context, mgr ctrl.Manager, node *corev1.Node) {
	logger := ctrl.LoggerFrom(ctx)

	r.embeddedRegistry.mutex.Lock()
	delete(r.embeddedRegistry.masterNodes, node.Name)
	r.embeddedRegistry.mutex.Unlock()
	logger.Info("Node deleted", "nodeName", node.Name)
	select {
	case <-mgr.Elected():
		// Only the leader should reconcile
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      "registry-node-" + node.Name + "-pki",
				Namespace: k8s.RegistryNamespace,
			},
		}
		_, err := r.Reconcile(ctx, req)
		if err != nil {
			logger.Error(err, "Reconcile failed")
		}
	default:
		logger.Info("Not the leader, skipping Reconcile")
	}
}

func (r *RegistryReconciler) handleModuleConfigChange(ctx context.Context, mgr ctrl.Manager, obj interface{}) {
	logger := ctrl.LoggerFrom(ctx)
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

	r.embeddedRegistry.mutex.Lock()
	r.embeddedRegistry.mc.Enabled = enabled
	r.embeddedRegistry.mc.Settings = settings
	r.embeddedRegistry.mutex.Unlock()
	logger.Info("ModuleConfig updated", "enabled", r.embeddedRegistry.mc.Enabled, "settings", r.embeddedRegistry.mc.Settings)

	select {
	case <-mgr.Elected():
		// Only the leader should reconcile
		for nodeName := range r.embeddedRegistry.masterNodes {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      "registry-node-" + nodeName + "-pki",
					Namespace: k8s.RegistryNamespace,
				},
			}
			_, err := r.Reconcile(ctx, req)
			if err != nil {
				logger.Error(err, "Reconcile failed")
			}
		}

	default:
		logger.Info("Not the leader, skipping Reconcile")
	}
}

func (r *RegistryReconciler) handleModuleConfigDelete(ctx context.Context) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Error(fmt.Errorf("ModuleConfig was deleted"), "ModuleConfig was deleted")
	r.embeddedRegistry.mutex.Lock()
	defer r.embeddedRegistry.mutex.Unlock()
	// Reset the ModuleConfig settings to default when deleted
	r.embeddedRegistry.mc.Enabled = false
	r.embeddedRegistry.mc.Settings = RegistryConfig{}
}
