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

package capicluster

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	cloudProviderSecretName      = "d8-node-manager-cloud-provider"
	cloudProviderSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	capiNamespace                = "d8-cloud-instance-manager"
)

func init() {
	register.RegisterController("capi-cluster-resources", &corev1.Secret{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, _ client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name:      cloudProviderSecretName,
				Namespace: cloudProviderSecretNamespace,
			}}}
		},
	))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != cloudProviderSecretName || req.Namespace != cloudProviderSecretNamespace {
		return ctrl.Result{}, nil
	}

	clusterConfig, err := r.readClusterConfiguration(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureCloudCluster(ctx, clusterConfig); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureStaticCluster(ctx, clusterConfig); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureCloudCluster(ctx context.Context, clusterConfig *clusterConfiguration) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("get cloud-provider secret: %w", err)
	}

	clusterName := string(secret.Data["capiClusterName"])
	clusterKind := string(secret.Data["capiClusterKind"])
	infraAPIVersion := string(secret.Data["capiClusterAPIVersion"])

	if clusterName == "" || clusterKind == "" {
		return nil
	}
	if infraAPIVersion == "" {
		infraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}

	infraAPIGroup := infraAPIVersion
	if idx := strings.LastIndex(infraAPIGroup, "/"); idx >= 0 {
		infraAPIGroup = infraAPIGroup[:idx]
	}

	commonLabels := map[string]interface{}{
		"heritage": "deckhouse",
		"module":   "node-manager",
		"app":      "capi-controller-manager",
	}

	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      clusterName,
			"namespace": capiNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterNetwork": map[string]interface{}{
				"pods":          map[string]interface{}{"cidrBlocks": []interface{}{clusterConfig.PodSubnetCIDR}},
				"services":      map[string]interface{}{"cidrBlocks": []interface{}{clusterConfig.ServiceSubnetCIDR}},
				"serviceDomain": clusterConfig.ClusterDomain,
			},
			"infrastructureRef": map[string]interface{}{
				"apiGroup": infraAPIGroup,
				"kind":     clusterKind,
				"name":     clusterName,
			},
			"controlPlaneRef": map[string]interface{}{
				"apiGroup": "infrastructure.cluster.x-k8s.io",
				"kind":     "DeckhouseControlPlane",
				"name":     clusterName + "-control-plane",
			},
		},
	}}

	if err := r.createIfNotExists(ctx, cluster); err != nil {
		return fmt.Errorf("create Cluster: %w", err)
	}

	mhc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineHealthCheck",
		"metadata": map[string]interface{}{
			"name":      clusterName + "-machine-health-check",
			"namespace": capiNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterName": clusterName,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": clusterName,
				},
			},
			"checks": map[string]interface{}{
				"nodeStartupTimeoutSeconds": int64(1200),
				"unhealthyNodeConditions": []interface{}{
					map[string]interface{}{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(300)},
					map[string]interface{}{"type": "Ready", "status": "False", "timeoutSeconds": int64(300)},
				},
			},
		},
	}}

	if err := r.createIfNotExists(ctx, mhc); err != nil {
		return fmt.Errorf("create MachineHealthCheck: %w", err)
	}

	logger.Info("ensured cloud CAPI cluster resources", "cluster", clusterName)
	return nil
}

func (r *Reconciler) ensureStaticCluster(ctx context.Context, clusterConfig *clusterConfiguration) error {
	logger := log.FromContext(ctx)

	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return fmt.Errorf("list NodeGroups: %w", err)
	}

	hasStatic := false
	for i := range ngList.Items {
		if ngList.Items[i].Spec.StaticInstances != nil {
			hasStatic = true
			break
		}
	}
	if !hasStatic {
		return nil
	}

	staticLabels := map[string]interface{}{
		"heritage": "deckhouse",
		"module":   "node-manager",
		"app":      "caps-controller-manager",
	}

	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      "static",
			"namespace": capiNamespace,
			"labels":    staticLabels,
		},
		"spec": map[string]interface{}{
			"clusterNetwork": map[string]interface{}{
				"pods":          map[string]interface{}{"cidrBlocks": []interface{}{clusterConfig.PodSubnetCIDR}},
				"services":      map[string]interface{}{"cidrBlocks": []interface{}{clusterConfig.ServiceSubnetCIDR}},
				"serviceDomain": clusterConfig.ClusterDomain,
			},
			"infrastructureRef": map[string]interface{}{
				"apiGroup": "infrastructure.cluster.x-k8s.io",
				"kind":     "StaticCluster",
				"name":     "static",
			},
			"controlPlaneRef": map[string]interface{}{
				"apiGroup": "infrastructure.cluster.x-k8s.io",
				"kind":     "DeckhouseControlPlane",
				"name":     "static-control-plane",
			},
		},
	}}

	if err := r.createIfNotExists(ctx, cluster); err != nil {
		return fmt.Errorf("create static Cluster: %w", err)
	}

	mhc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineHealthCheck",
		"metadata": map[string]interface{}{
			"name":      "static-machine-health-check",
			"namespace": capiNamespace,
			"labels":    staticLabels,
		},
		"spec": map[string]interface{}{
			"clusterName": "static",
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": "static",
				},
			},
			"checks": map[string]interface{}{
				"nodeStartupTimeoutSeconds": int64(1200),
				"unhealthyNodeConditions": []interface{}{
					map[string]interface{}{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(2147483647)},
				},
			},
		},
	}}

	if err := r.createIfNotExists(ctx, mhc); err != nil {
		return fmt.Errorf("create static MachineHealthCheck: %w", err)
	}

	logger.Info("ensured static CAPI cluster resources")
	return nil
}

func (r *Reconciler) createIfNotExists(ctx context.Context, obj *unstructured.Unstructured) error {
	err := r.Client.Create(ctx, obj)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

type clusterConfiguration struct {
	PodSubnetCIDR     string `json:"podSubnetCIDR"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	ClusterDomain     string `json:"clusterDomain"`
}

func (r *Reconciler) readClusterConfiguration(ctx context.Context) (*clusterConfiguration, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      clusterConfigSecretName,
		Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		// Try string data (some clusters store it differently).
		rawStr, ok := secret.Data["cluster-configuration.yaml"]
		if !ok {
			return nil, fmt.Errorf("cluster-configuration secret missing cluster-configuration.yaml key")
		}
		// Decode base64 if needed.
		decoded, err := base64.StdEncoding.DecodeString(string(rawStr))
		if err != nil {
			raw = rawStr
		} else {
			raw = decoded
		}
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
	}

	return cfg, nil
}

// cacheConfig is handled in common/cache.go:
// - Secret in kube-system with label name=d8-cluster-configuration is already cached
// - Secret d8-node-manager-cloud-provider is watched as primary object
// - Cluster/MHC are created in d8-cloud-instance-manager (unstructured, no cache needed)
