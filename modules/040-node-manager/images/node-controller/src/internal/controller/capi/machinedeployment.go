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

package capi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	sigsyaml "sigs.k8s.io/yaml"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	engineCAPI = "CAPI"
	engineMCM  = "MCM"

	// mdCleanupFinalizer holds the NodeGroup until its MachineDeployments are deleted.
	mdCleanupFinalizer = "node-manager.deckhouse.io/capi-md-cleanup"
)

func init() {
	register.RegisterController("capi-machine-deployment", &deckhousev1.NodeGroup{}, &MachineDeploymentReconciler{})
}

type MachineDeploymentReconciler struct {
	BaseWithReader
}

func (r *MachineDeploymentReconciler) SetupWatches(w register.Watcher) {
	mcmMD := &unstructured.Unstructured{}
	mcmMD.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeployment",
	})
	// Re-enqueue only on spec/generation changes — status updates (e.g. from
	// capi-controller-manager) must not trigger a re-apply, otherwise reconcile loops.
	w.Watches(mcmMD, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup),
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	w.Watches(&capiv1beta2.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup),
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))
}

func mdToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.GetLabels()["node-group"]
	if !ok || ng == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ng}}}
}

func (r *MachineDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get NodeGroup: %w", err)
	}

	if !ng.DeletionTimestamp.IsZero() {
		if err := r.cleanupMachineDeployments(ctx, ng.Name); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.removeFinalizer(ctx, ng); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureFinalizer(ctx, ng); err != nil {
		return ctrl.Result{}, err
	}

	switch ng.Spec.NodeType {
	case deckhousev1.NodeTypeCloudEphemeral:
		switch ng.Status.Engine {
		case engineCAPI:
			if err := r.reconcileCloudMDsRendered(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		case engineMCM:
			if err := r.reconcileCloudMCMs(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		default:
			logger.V(1).Info("skipping: engine not set or unsupported", "engine", ng.Status.Engine)
		}
	case deckhousev1.NodeTypeStatic, deckhousev1.NodeTypeCloudStatic:
		if ng.Spec.StaticInstances != nil {
			if err := r.reconcileStaticMDRendered(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *MachineDeploymentReconciler) ensureFinalizer(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	if controllerutil.ContainsFinalizer(ng, mdCleanupFinalizer) {
		return nil
	}
	updated := ng.DeepCopy()
	controllerutil.AddFinalizer(updated, mdCleanupFinalizer)
	if err := r.Client.Patch(ctx, updated, client.MergeFrom(ng)); err != nil {
		return fmt.Errorf("add finalizer to NodeGroup %s: %w", ng.Name, err)
	}
	*ng = *updated
	return nil
}

func (r *MachineDeploymentReconciler) removeFinalizer(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	if !controllerutil.ContainsFinalizer(ng, mdCleanupFinalizer) {
		return nil
	}
	updated := ng.DeepCopy()
	controllerutil.RemoveFinalizer(updated, mdCleanupFinalizer)
	if err := r.Client.Patch(ctx, updated, client.MergeFrom(ng)); err != nil {
		return fmt.Errorf("remove finalizer from NodeGroup %s: %w", ng.Name, err)
	}
	*ng = *updated
	return nil
}

// cleanupMachineDeployments deletes the CAPI and MCM MachineDeployments belonging to the NodeGroup.
// The actual node drain is driven asynchronously by capi/caps-controller-manager via their own
// finalizers, so this only issues the deletes and returns — the NodeGroup is not held waiting for it.
func (r *MachineDeploymentReconciler) cleanupMachineDeployments(ctx context.Context, ngName string) error {
	logger := log.FromContext(ctx)

	gvks := []schema.GroupVersionKind{
		{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeploymentList"},
		{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeploymentList"},
	}

	for _, gvk := range gvks {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		if err := r.Client.List(ctx, list,
			client.InNamespace(common.MachineNamespace),
			client.MatchingLabels{"node-group": ngName},
		); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return fmt.Errorf("list %s for NodeGroup %s: %w", gvk.Kind, ngName, err)
		}

		for i := range list.Items {
			md := &list.Items[i]
			if !md.GetDeletionTimestamp().IsZero() {
				continue
			}
			if err := r.Client.Delete(ctx, md); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete MachineDeployment %s: %w", md.GetName(), err)
			}
			logger.V(1).Info("deleted MachineDeployment for removed NodeGroup", "name", md.GetName(), "ng", ngName)
		}
	}

	return nil
}

// buildStaticMD renders the cluster.x-k8s.io/v1beta2 MachineDeployment for a
// Static/CloudStatic NodeGroup. Extracted so the live reconcileStaticMD and the
// rendered-cutover reconcileStaticMDRendered build byte-identical objects.
func buildStaticMD(ng *deckhousev1.NodeGroup) *unstructured.Unstructured {
	var replicas int32
	if ng.Spec.StaticInstances.Count != nil {
		replicas = *ng.Spec.StaticInstances.Count
	}

	commonLabels := map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": ng.Name,
		"app":        "caps-controller",
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineDeployment",
		"metadata": map[string]interface{}{
			"name":      ng.Name,
			"namespace": common.MachineNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterName": "static",
			"replicas":    int64(replicas),
			"rollout": map[string]interface{}{
				"strategy": map[string]interface{}{
					"type": "RollingUpdate",
					"rollingUpdate": map[string]interface{}{
						"maxSurge":       int64(1),
						"maxUnavailable": int64(0),
					},
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"cluster.x-k8s.io/cluster-name":    "static",
						"cluster.x-k8s.io/deployment-name": ng.Name,
					},
				},
				"spec": map[string]interface{}{
					"clusterName": "static",
					"bootstrap": map[string]interface{}{
						"dataSecretName": fmt.Sprintf("manual-bootstrap-for-%s", ng.Name),
					},
					"infrastructureRef": map[string]interface{}{
						"apiGroup": "infrastructure.cluster.x-k8s.io",
						"kind":     "StaticMachineTemplate",
						"name":     ng.Name,
					},
				},
			},
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name":    "static",
					"cluster.x-k8s.io/deployment-name": ng.Name,
				},
			},
		},
	}}
}

type cloudProviderConfig struct {
	capiClusterName               string
	capiMachineTemplateKind       string
	capiMachineTemplateAPIVersion string
	zones                         []string
}

func (r *MachineDeploymentReconciler) readCloudProviderConfig(ctx context.Context) (*cloudProviderConfig, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return &cloudProviderConfig{}, nil
		}
		return nil, fmt.Errorf("get cloud-provider secret: %w", err)
	}

	cfg := &cloudProviderConfig{
		capiClusterName:               string(secret.Data["capiClusterName"]),
		capiMachineTemplateKind:       string(secret.Data["capiMachineTemplateKind"]),
		capiMachineTemplateAPIVersion: string(secret.Data["capiMachineTemplateAPIVersion"]),
	}
	if cfg.capiMachineTemplateAPIVersion == "" {
		cfg.capiMachineTemplateAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}
	if raw := secret.Data["zones"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg.zones)
	}
	return cfg, nil
}

func (r *MachineDeploymentReconciler) readClusterUUID(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: clusterUUIDConfigMapName, Namespace: clusterUUIDConfigMapNS,
	}, cm); err != nil {
		return "", fmt.Errorf("get cluster-uuid configmap: %w", err)
	}
	return cm.Data["cluster-uuid"], nil
}

type mdClusterConfiguration struct {
	Cloud struct {
		Prefix string `json:"prefix"`
	} `json:"cloud"`
}

func (r *MachineDeploymentReconciler) readInstancePrefix(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return "", nil
		}
		return "", fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		decoded = raw
	}

	cfg := &mdClusterConfiguration{}
	if err := sigsyaml.Unmarshal(decoded, cfg); err != nil {
		return "", fmt.Errorf("unmarshal cluster configuration: %w", err)
	}
	return cfg.Cloud.Prefix, nil
}

func getMinMax(ng *deckhousev1.NodeGroup) (int32, int32) {
	if ng.Spec.StaticInstances != nil && ng.Spec.StaticInstances.Count != nil {
		count := *ng.Spec.StaticInstances.Count
		return count, count
	}
	if ng.Spec.CloudInstances != nil {
		return ng.Spec.CloudInstances.MinPerZone, ng.Spec.CloudInstances.MaxPerZone
	}
	return 0, 0
}

func calculateReplicas(current, min, max int32) int32 {
	switch {
	case min >= max:
		return max
	case current == 0:
		return min
	case current <= min:
		return min
	case current > max:
		return max
	default:
		return current
	}
}

func sha256Hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:8]
}

func intOrDefault(ptr *int32, def int) int {
	if ptr != nil {
		return int(*ptr)
	}
	return def
}

func serializeNodeGroupLabels(ng *deckhousev1.NodeGroup) string {
	merged := make(map[string]string)
	if ng.Spec.NodeTemplate != nil {
		for k, v := range ng.Spec.NodeTemplate.Labels {
			merged[k] = v
		}
	}
	merged["node.deckhouse.io/group"] = ng.Name
	merged["node.deckhouse.io/type"] = string(ng.Spec.NodeType)
	merged["node-role.kubernetes.io/"+ng.Name] = ""
	return labels.FormatLabels(merged)
}

func serializeNodeGroupTaints(ng *deckhousev1.NodeGroup) string {
	if ng.Spec.NodeTemplate == nil || len(ng.Spec.NodeTemplate.Taints) == 0 {
		return ""
	}
	res := make([]string, 0, len(ng.Spec.NodeTemplate.Taints))
	for _, taint := range ng.Spec.NodeTemplate.Taints {
		res = append(res, taint.ToString())
	}
	sort.Strings(res)
	return strings.Join(res, ",")
}
