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
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("capi-cluster-resources", &corev1.Secret{}, &ClusterReconciler{})
}

type ClusterReconciler struct {
	BaseWithReader
}

func isProviderTemplateSecret(namespace, name string) bool {
	return namespace == providerTemplateSecretNamespace &&
		strings.HasPrefix(name, "d8-cloud-provider-") &&
		strings.HasSuffix(name, "-capi")
}

func isClusterReconcileRequest(req ctrl.Request) bool {
	if req.Namespace == cloudProviderSecretNamespace &&
		req.Name == cloudProviderSecretName {
		return true
	}

	return isProviderTemplateSecret(req.Namespace, req.Name)
}

func (r *ClusterReconciler) SetupWatches(w register.Watcher) {
	// WithEventFilter is controller-wide, so it also covers the NodeGroup watch below.
	// NodeGroup events must always pass — on a static cluster the cloud-provider Secret may not
	// exist, and NodeGroup is the only trigger for ensureStaticCluster. The Secret (primary For)
	// is filtered down to the cloud-provider and CAPI provider-template Secrets.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if _, ok := obj.(*deckhousev1.NodeGroup); ok {
			return true
		}

		if obj.GetNamespace() == cloudProviderSecretNamespace &&
			obj.GetName() == cloudProviderSecretName {
			return true
		}

		return isProviderTemplateSecret(obj.GetNamespace(), obj.GetName())
	}))

	// Re-enqueue only on spec/generation changes — NodeGroup status updates must not trigger
	// a no-op re-ensure, otherwise the createIfNotExists path logs on every status bump.
	w.Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, _ client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name:      cloudProviderSecretName,
				Namespace: cloudProviderSecretNamespace,
			}}}
		},
	), builder.WithPredicates(predicate.GenerationChangedPredicate{}))
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !isClusterReconcileRequest(req) {
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

func (r *ClusterReconciler) ensureCloudCluster(
	ctx context.Context,
	clusterConfig *clusterConfiguration,
) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name:      cloudProviderSecretName,
		Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("get cloud-provider secret: %w", err)
	}

	clusterName := string(secret.Data["capiClusterName"])
	clusterKind := string(secret.Data["capiClusterKind"])
	infraAPIVersion := string(secret.Data["capiClusterAPIVersion"])
	cloudType := string(secret.Data["type"])

	if clusterName == "" || clusterKind == "" {
		return nil
	}

	if cloudType == "" {
		return fmt.Errorf("cloud-provider secret has no type")
	}

	if infraAPIVersion == "" {
		infraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}

	provider := map[string]any{}
	if rawProvider := secret.Data[cloudType]; len(rawProvider) > 0 {
		if err := json.Unmarshal(rawProvider, &provider); err != nil {
			return fmt.Errorf(
				"decode %s provider configuration: %w",
				cloudType,
				err,
			)
		}
	}

	clusterTemplateContext := map[string]any{
		"name":          clusterName,
		"namespace":     capiNamespace,
		"podSubnet":     clusterConfig.PodSubnetCIDR,
		"serviceSubnet": clusterConfig.ServiceSubnetCIDR,
		"domain":        clusterConfig.ClusterDomain,
	}

	if err := r.ensureProviderInfrastructure(
		ctx,
		cloudType,
		provider,
		clusterTemplateContext,
		infraAPIVersion,
		clusterKind,
		clusterName,
	); err != nil {
		return err
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
				"pods": map[string]interface{}{
					"cidrBlocks": []interface{}{clusterConfig.PodSubnetCIDR},
				},
				"services": map[string]interface{}{
					"cidrBlocks": []interface{}{clusterConfig.ServiceSubnetCIDR},
				},
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
					map[string]interface{}{
						"type":           "Ready",
						"status":         "Unknown",
						"timeoutSeconds": int64(300),
					},
					map[string]interface{}{
						"type":           "Ready",
						"status":         "False",
						"timeoutSeconds": int64(300),
					},
				},
			},
		},
	}}

	if err := r.createIfNotExists(ctx, mhc); err != nil {
		return fmt.Errorf("create MachineHealthCheck: %w", err)
	}

	logger.V(1).Info(
		"ensured cloud CAPI cluster resources",
		"cluster",
		clusterName,
	)

	return nil
}

func (r *ClusterReconciler) ensureProviderInfrastructure(
	ctx context.Context,
	cloudType string,
	provider map[string]any,
	cluster map[string]any,
	expectedAPIVersion string,
	expectedKind string,
	expectedName string,
) error {
	data, found, err := r.readProviderTemplateIfPresent(
		ctx,
		cloudType,
		engineCAPITemplates,
		clusterTemplateContractKey,
	)
	if err != nil {
		return fmt.Errorf(
			"read %s cluster-template contract: %w",
			cloudType,
			err,
		)
	}

	if !found {
		// The provider has not migrated to the new cluster-template contract.
		// Its infrastructure resource is still rendered by Helm.
		return nil
	}

	contract, err := parseClusterTemplateContract(data)
	if err != nil {
		return fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}

	objects, err := renderClusterTemplate(contract, provider, cluster)
	if err != nil {
		return fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}

	infrastructureCount := 0

	// Validate every rendered object before applying anything. This prevents
	// partially applying a template that contains a valid Secret followed by
	// an invalid or missing infrastructure object.
	for _, object := range objects {
		if object.GetNamespace() != capiNamespace {
			return fmt.Errorf(
				"provider infrastructure %s %s must be in namespace %s",
				object.GetKind(),
				object.GetName(),
				capiNamespace,
			)
		}

		isInfrastructureObject :=
			object.GetAPIVersion() == expectedAPIVersion &&
				object.GetKind() == expectedKind

		isAuxiliarySecret :=
			object.GetAPIVersion() == "v1" &&
				object.GetKind() == "Secret"

		if !isInfrastructureObject && !isAuxiliarySecret {
			return fmt.Errorf(
				"cluster-template cannot create %s %s",
				object.GetAPIVersion(),
				object.GetKind(),
			)
		}

		if !isInfrastructureObject {
			continue
		}

		if object.GetName() != expectedName {
			return fmt.Errorf(
				"provider infrastructure %s must be named %q, got %q",
				expectedKind,
				expectedName,
				object.GetName(),
			)
		}

		infrastructureCount++
	}

	if infrastructureCount == 0 {
		return fmt.Errorf(
			"cluster-template did not render expected %s %s",
			expectedKind,
			expectedName,
		)
	}

	if infrastructureCount > 1 {
		return fmt.Errorf(
			"cluster-template rendered more than one %s",
			expectedKind,
		)
	}

	for _, object := range objects {
		labels := object.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}

		labels["heritage"] = "deckhouse"
		labels["module"] = "node-manager"
		object.SetLabels(labels)

		if err := r.Client.Patch(
			ctx,
			object,
			client.Apply,
			client.FieldOwner("node-controller"),
			client.ForceOwnership,
		); err != nil {
			return fmt.Errorf(
				"apply provider infrastructure %s %s: %w",
				object.GetKind(),
				object.GetName(),
				err,
			)
		}
	}

	return nil
}

func (r *ClusterReconciler) ensureStaticCluster(
	ctx context.Context,
	clusterConfig *clusterConfiguration,
) error {
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
				"pods": map[string]interface{}{
					"cidrBlocks": []interface{}{clusterConfig.PodSubnetCIDR},
				},
				"services": map[string]interface{}{
					"cidrBlocks": []interface{}{clusterConfig.ServiceSubnetCIDR},
				},
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
					map[string]interface{}{
						"type":           "Ready",
						"status":         "Unknown",
						"timeoutSeconds": int64(2147483647),
					},
				},
			},
		},
	}}

	if err := r.createIfNotExists(ctx, mhc); err != nil {
		return fmt.Errorf("create static MachineHealthCheck: %w", err)
	}

	logger.V(1).Info("ensured static CAPI cluster resources")

	return nil
}

func (r *ClusterReconciler) createIfNotExists(
	ctx context.Context,
	obj *unstructured.Unstructured,
) error {
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

func (r *ClusterReconciler) readClusterConfiguration(
	ctx context.Context,
) (*clusterConfiguration, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name:      clusterConfigSecretName,
		Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(
			"cluster-configuration secret missing cluster-configuration.yaml key",
		)
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
	}

	return cfg, nil
}
