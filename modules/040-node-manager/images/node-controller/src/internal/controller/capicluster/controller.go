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

// Package capicluster creates the CAPI Cluster and MachineHealthCheck objects
// (cluster.x-k8s.io/v1beta2) from the cloud-provider registration secret. It is
// the controller-runtime port of the create_capi_cluster_resources hook.
//
// Owning these objects in a controller (instead of the helm release) keeps them
// off helm's apply path, so a transient conversion-webhook failure on first
// install does not stall helm: the reconcile simply retries until the webhook
// backend is up. Creation is idempotent (AlreadyExists is ignored).
package capicluster

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	cloudProviderSecretNamespace = "kube-system"
	cloudProviderSecretName      = "d8-node-manager-cloud-provider"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretKey       = "cluster-configuration.yaml"

	defaultInfraAPIGroup    = "infrastructure.cluster.x-k8s.io"
	controlPlaneKind        = "DeckhouseControlPlane"
	nodeStartupTimeoutSecs  = 1200
	unhealthyTimeoutSeconds = 300
)

var (
	clusterGVK = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"}
	mhcGVK     = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineHealthCheck"}
)

type clusterConfiguration struct {
	PodSubnetCIDR     string `json:"podSubnetCIDR"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	ClusterDomain     string `json:"clusterDomain"`
}

type reconciler struct {
	register.Base
}

var _ register.Reconciler = (*reconciler)(nil)

func (r *reconciler) SetupWatches(w register.Watcher) {
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == cloudProviderSecretNamespace && obj.GetName() == cloudProviderSecretName
	}))
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var secret corev1.Secret
	if err := r.Client.Get(ctx, req.NamespacedName, &secret); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clusterName := string(secret.Data["capiClusterName"])
	clusterKind := string(secret.Data["capiClusterKind"])
	if clusterName == "" || clusterKind == "" {
		return ctrl.Result{}, nil
	}

	infraAPIGroup := defaultInfraAPIGroup
	if v := string(secret.Data["capiClusterAPIVersion"]); v != "" {
		infraAPIGroup = apiGroupFromAPIVersion(v)
	}

	clusterConfig, err := r.readClusterConfiguration(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("read cluster configuration: %w", err)
	}

	cluster := buildCluster(clusterName, clusterKind, infraAPIGroup, clusterConfig)
	mhc := buildMachineHealthCheck(clusterName)

	for _, obj := range []*unstructured.Unstructured{cluster, mhc} {
		if err := r.Client.Create(ctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, fmt.Errorf("create %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) readClusterConfiguration(ctx context.Context) (clusterConfiguration, error) {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: cloudProviderSecretNamespace, Name: clusterConfigSecretName}
	if err := r.Client.Get(ctx, key, &secret); err != nil {
		return clusterConfiguration{}, fmt.Errorf("get secret %s: %w", clusterConfigSecretName, err)
	}

	raw, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return clusterConfiguration{}, nil
	}

	var config clusterConfiguration
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return clusterConfiguration{}, fmt.Errorf("unmarshal %s: %w", clusterConfigSecretKey, err)
	}
	return config, nil
}

func apiGroupFromAPIVersion(apiVersion string) string {
	if group, _, found := strings.Cut(apiVersion, "/"); found {
		return group
	}
	return apiVersion
}

func commonLabels() map[string]interface{} {
	return map[string]interface{}{
		"heritage": "deckhouse",
		"module":   "node-manager",
		"app":      "capi-controller-manager",
	}
}

func buildCluster(name, infraKind, infraAPIGroup string, config clusterConfiguration) *unstructured.Unstructured {
	cluster := ngcommon.NewUnstructured(clusterGVK)
	cluster.Object["metadata"] = map[string]interface{}{
		"name":      name,
		"namespace": ngcommon.MachineNamespace,
		"labels":    commonLabels(),
	}
	cluster.Object["spec"] = map[string]interface{}{
		"clusterNetwork": map[string]interface{}{
			"pods":          map[string]interface{}{"cidrBlocks": []interface{}{config.PodSubnetCIDR}},
			"services":      map[string]interface{}{"cidrBlocks": []interface{}{config.ServiceSubnetCIDR}},
			"serviceDomain": config.ClusterDomain,
		},
		"infrastructureRef": map[string]interface{}{
			"apiGroup": infraAPIGroup,
			"kind":     infraKind,
			"name":     name,
		},
		"controlPlaneRef": map[string]interface{}{
			"apiGroup": defaultInfraAPIGroup,
			"kind":     controlPlaneKind,
			"name":     name + "-control-plane",
		},
	}
	return cluster
}

func buildMachineHealthCheck(clusterName string) *unstructured.Unstructured {
	mhc := ngcommon.NewUnstructured(mhcGVK)
	mhc.Object["metadata"] = map[string]interface{}{
		"name":      clusterName + "-machine-health-check",
		"namespace": ngcommon.MachineNamespace,
		"labels":    commonLabels(),
	}
	mhc.Object["spec"] = map[string]interface{}{
		"clusterName":               clusterName,
		"nodeStartupTimeoutSeconds": int64(nodeStartupTimeoutSecs),
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"cluster.x-k8s.io/cluster-name": clusterName,
			},
		},
		"unhealthyNodeConditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(unhealthyTimeoutSeconds)},
			map[string]interface{}{"type": "Ready", "status": "False", "timeoutSeconds": int64(unhealthyTimeoutSeconds)},
		},
	}
	return mhc
}

func init() {
	register.RegisterController("create-capi-cluster-resources", &corev1.Secret{}, &reconciler{})
}
