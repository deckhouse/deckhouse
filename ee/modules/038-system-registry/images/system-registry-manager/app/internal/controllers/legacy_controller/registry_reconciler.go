/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	httpclient "embeded-registry-manager/internal/utils/http_client"
	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
)

type RegistryReconciler struct {
	client.Client
	APIReader        client.Reader // To use for objects without cache
	KubeClient       *kubernetes.Clientset
	Recorder         record.EventRecorder
	HttpClient       *httpclient.Client
	embeddedRegistry embeddedRegistry
	deletedSecrets   sync.Map
}

var nodePKISecretRegex = regexp.MustCompile(`^registry-node-(.*)-pki$`)

// SetupWithManager sets up the controller with the Manager to watch for changes in both Pods and Secrets.
func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	// Set up the field indexer to index Pods by spec.nodeName
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
		pod := obj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return fmt.Errorf("failed to set up index on spec.nodeName: %w", err)
	}
	r.embeddedRegistry.masterNodes = make(map[string]k8s.MasterNode)

	// Set up moduleConfig informer
	moduleConfig := &unstructured.Unstructured{}
	moduleConfig.SetAPIVersion(k8s.ModuleConfigApiVersion)
	moduleConfig.SetKind(k8s.ModuleConfigKind)

	moduleConfigInformer, err := mgr.GetCache().GetInformer(ctx, moduleConfig)
	if err != nil {
		return fmt.Errorf("unable to get informer for ModuleConfig: %w", err)
	}

	// Add event handler for ModuleConfig
	_, err = moduleConfigInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				unstructuredObj, ok := obj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigChange(ctx, mgr, obj)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				unstructuredObj, ok := newObj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigChange(ctx, mgr, newObj)
				}

			},
			DeleteFunc: func(obj interface{}) {
				unstructuredObj, ok := obj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigDelete(ctx)
				}
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to add event handler for ModuleConfig: %w", err)
	}

	// Set up Node informer
	nodeInformer, err := mgr.GetCache().GetInformer(ctx, &corev1.Node{})
	if err != nil {
		return fmt.Errorf("unable to get informer for Node: %w", err)
	}
	// #TODO
	// Add event handler for Node
	_, err = nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if node, ok := obj.(*corev1.Node); ok {
				if hasMasterLabel(node) {
					r.handleNodeAdd(ctx, mgr, node)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode, okOld := oldObj.(*corev1.Node)
			newNode, okNew := newObj.(*corev1.Node)
			if okOld && okNew {
				oldIsMaster := hasMasterLabel(oldNode)
				newIsMaster := hasMasterLabel(newNode)
				switch {
				case !oldIsMaster && newIsMaster:
					r.handleNodeAdd(ctx, mgr, newNode)
				case oldIsMaster && !newIsMaster:
					r.handleNodeDelete(ctx, mgr, oldNode)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if node, ok := obj.(*corev1.Node); ok {
				if hasMasterLabel(node) {
					r.handleNodeDelete(ctx, mgr, node)
				}
			}
		},
	})
	if err != nil {
		return fmt.Errorf("unable to add event handler for Node: %w", err)
	}

	secretsToWatch := []string{
		"registry-user-ro",
		"registry-user-rw",
		"registry-pki",
	}

	// Watch for changes in Secrets
	err = ctrl.NewControllerManagedBy(mgr).
		Named("embedded-registry-controller").
		For(&corev1.Secret{},
			builder.WithPredicates(
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					objectName := object.GetName()

					for _, name := range secretsToWatch {
						if name == objectName {
							return true
						}
					}

					return nodePKISecretRegex.MatchString(objectName)
				}),
			),
		).
		//WatchesRawSource(oneshotSource("registry-pki", "d8-system")).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)

	if err != nil {
		return fmt.Errorf("unable to complete controller: %w", err)
	}

	return nil
}

func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	// Lock the mutex for the embedded registry struct to prevent simultaneous writes
	r.embeddedRegistry.mutex.Lock()
	defer r.embeddedRegistry.mutex.Unlock()

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	switch {
	// Check if the secret is the registry-pki secret
	case req.NamespacedName.Name == "registry-pki":
		err := r.handleCAPKI(ctx, req, secret)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	// Check if the secret is the registry-node-*-pki secret
	case nodePKISecretRegex.MatchString(req.NamespacedName.Name):
		nodeName := strings.TrimPrefix(strings.TrimSuffix(req.NamespacedName.Name, "-pki"), "registry-node-")

		err := r.handleNodePKI(ctx, req, nodeName, secret)

		if err != nil {
			return ctrl.Result{}, err
		}

	// Check if the secret is the registry-user-rw secret
	case req.NamespacedName.Name == "registry-user-rw":
		_, err := r.handleRegistryUser(ctx, req, "registry-user-rw", &r.embeddedRegistry.registryRwUser, secret)
		if err != nil {
			return ctrl.Result{}, err
		}
	// Check if the secret is the registry-user-ro secret
	case req.NamespacedName.Name == "registry-user-ro":
		_, err := r.handleRegistryUser(ctx, req, "registry-user-ro", &r.embeddedRegistry.registryRoUser, secret)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync the registry static pods
	var response []byte
	var reconcileErr error
	var nodesToSync []k8s.MasterNode

	if r.embeddedRegistry.mc.Settings.Mode == "Detached" {
		firstNode := k8s.GetFirstCreatedNodeForSync(r.embeddedRegistry.masterNodes)
		nodesToSync = []k8s.MasterNode{*firstNode}
		logger.Info("Detached mode, syncing registry only on the first created master node", "node", firstNode.Name)
	} else {
		for _, node := range r.embeddedRegistry.masterNodes {
			nodesToSync = append(nodesToSync, node)
		}
	}

	for _, node := range nodesToSync {
		if r.embeddedRegistry.mc.Settings.Mode == "Direct" {
			response, reconcileErr = r.deleteNodeRegistry(ctx, node.Name)
		} else {
			response, reconcileErr = r.syncRegistryStaticPods(ctx, node)
		}

		if reconcileErr != nil {
			logger.Info("Failed to reconcile registry", "node", node.Name, "error", reconcileErr)
		} else {
			logger.Info("Reconciled registry", "node", node.Name, "response", string(response))
		}
	}
	// #TODO

	return ctrl.Result{}, nil
}

// extractModuleConfigFieldsFromObject extracts the 'enabled' and 'settings' fields from the ModuleConfig CR
func (r *RegistryReconciler) extractModuleConfigFieldsFromObject(cr *unstructured.Unstructured) (bool, map[string]interface{}, error) {
	// Extract the 'enabled' field from the ModuleConfig CR
	enabled, found, err := unstructured.NestedBool(cr.Object, "spec", "enabled")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'enabled' not found or failed to parse: %w", err)
	}
	// Extract the 'settings' field from the ModuleConfig CR
	settings, found, err := unstructured.NestedMap(cr.Object, "spec", "settings")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'settings' not found or failed to parse: %w", err)
	}
	return enabled, settings, nil
}
