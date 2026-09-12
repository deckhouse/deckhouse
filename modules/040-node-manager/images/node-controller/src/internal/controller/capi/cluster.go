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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
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
	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("capi-cluster-resources", &corev1.Secret{}, &ClusterReconciler{})
}

type ClusterReconciler struct {
	BaseWithReader
}

var clusterReconcileRequest = []reconcile.Request{{NamespacedName: types.NamespacedName{
	Name:      cloudProviderSecretName,
	Namespace: cloudProviderSecretNamespace,
}}}

func isProviderTemplateSecret(namespace, name string) bool {
	return namespace == providerTemplateSecretNamespace &&
		strings.HasPrefix(name, "d8-cloud-provider-") &&
		strings.HasSuffix(name, "-capi")
}

func isClusterReconcileRequest(req ctrl.Request) bool {
	if req.Namespace == cloudProviderSecretNamespace &&
		(req.Name == cloudProviderSecretName || req.Name == clusterConfigSecretName) {
		return true
	}

	return isProviderTemplateSecret(req.Namespace, req.Name)
}

// ForPredicates limits the primary Secret watch to inputs that can change cloud
// cluster resources. Provider template Secrets are matched by name because the
// active provider is discovered at runtime.
func (r *ClusterReconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return isClusterReconcileRequest(ctrl.Request{NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}})
	})}
}

func (r *ClusterReconciler) SetupWatches(w register.Watcher) {
	enqueue := handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, _ client.Object) []reconcile.Request {
			return clusterReconcileRequest
		},
	)

	// NodeGroup status updates do not affect static cluster resources.
	w.Watches(&deckhousev1.NodeGroup{}, enqueue,
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))

	// Cluster templates may depend on control-plane endpoints.
	w.Watches(&corev1.Pod{}, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == "kube-system" &&
				obj.GetLabels()["component"] == "kube-apiserver" &&
				obj.GetLabels()["tier"] == "control-plane"
		}),
	))
	w.Watches(&discoveryv1.EndpointSlice{}, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == "default" && obj.GetName() == "kubernetes"
		}),
	))

	// Cluster templates may use the global cluster prefix to name provider resources.
	moduleConfig := newUnstructured("deckhouse.io", "v1alpha1", "ModuleConfig")
	w.Watches(moduleConfig, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetName() == clusterprefix.GlobalModuleConfigName
		}),
	))
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

func (r *ClusterReconciler) ensureCloudCluster(ctx context.Context, clusterConfig *clusterConfiguration) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("get cloud-provider secret: %w", err)
	}

	cloudProvider := decodeCloudProviderSecret(secret.Data)
	clusterName, _ := cloudProvider["capiClusterName"].(string)
	clusterKind, _ := cloudProvider["capiClusterKind"].(string)
	infraAPIVersion, _ := cloudProvider["capiClusterAPIVersion"].(string)
	cloudType, _ := cloudProvider["type"].(string)

	if clusterName == "" || clusterKind == "" {
		return nil
	}

	if cloudType == "" {
		return fmt.Errorf("cloud-provider secret has no type")
	}

	if infraAPIVersion == "" {
		infraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}

	provider, _ := cloudProvider[cloudType].(map[string]interface{})
	if provider == nil {
		provider = map[string]interface{}{}
	}

	if err := r.ensureProviderInfrastructure(
		ctx,
		cloudType,
		provider,
		clusterConfig,
		infraAPIVersion,
		clusterKind,
		clusterName,
	); err != nil {
		return err
	}

	controlPlane := deckhouseControlPlane(clusterName)
	if err := r.applyClusterObject(ctx, controlPlane); err != nil {
		return fmt.Errorf("apply DeckhouseControlPlane %s: %w", controlPlane.GetName(), err)
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

	logger.V(1).Info("ensured cloud CAPI cluster resources", "cluster", clusterName)
	return nil
}

func (r *ClusterReconciler) ensureProviderInfrastructure(
	ctx context.Context,
	cloudType string,
	provider map[string]any,
	clusterConfig *clusterConfiguration,
	expectedAPIVersion string,
	expectedKind string,
	expectedName string,
) error {
	data, err := r.readProviderTemplate(
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

	contract, err := parseClusterTemplateContract(data)
	if err != nil {
		return fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}

	templateContext, err := r.buildClusterTemplateContext(
		ctx,
		provider,
		clusterConfig,
		expectedName,
	)
	if err != nil {
		return fmt.Errorf("build %s cluster-template context: %w", cloudType, err)
	}

	objects, err := renderClusterTemplate(contract, templateContext)
	if err != nil {
		return fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}

	if err := validateClusterTemplateObjects(
		objects,
		expectedAPIVersion,
		expectedKind,
		expectedName,
	); err != nil {
		return fmt.Errorf("cloud provider %s: %w", cloudType, err)
	}

	for _, object := range objects {
		if err := r.applyClusterObject(ctx, object); err != nil {
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

func (r *ClusterReconciler) applyClusterObject(ctx context.Context, object *unstructured.Unstructured) error {
	prepareClusterTemplateObject(object)
	if err := r.Client.Patch(
		ctx,
		object,
		client.Apply,
		client.FieldOwner("node-controller"),
		client.ForceOwnership,
	); err != nil {
		return err
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(object.GroupVersionKind())
	// Provider kinds are discovered at runtime, and credential Secrets do not match
	// node-controller's cache label selector. Read the just-applied object directly.
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(object), current); err != nil {
		return fmt.Errorf("read applied object for Helm handover: %w", err)
	}
	original := current.DeepCopy()
	if !removeLegacyHelmMetadata(current) {
		return nil
	}
	if err := r.Client.Patch(ctx, current, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("remove Helm ownership metadata: %w", err)
	}
	return nil
}

func deckhouseControlPlane(clusterName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
		"kind":       "DeckhouseControlPlane",
		"metadata": map[string]interface{}{
			"name":      clusterName + "-control-plane",
			"namespace": capiNamespace,
			"labels": map[string]interface{}{
				"app": "capi-controller-manager",
			},
		},
	}}
}

func (r *ClusterReconciler) buildClusterTemplateContext(
	ctx context.Context,
	provider map[string]any,
	clusterConfig *clusterConfiguration,
	clusterName string,
) (clusterTemplateContext, error) {
	endpoints, err := (&bashiblecontext.Service{
		Client: r.Client,
		Reader: r.APIReader,
	}).ReadEndpoints(ctx)
	if err != nil {
		return clusterTemplateContext{}, fmt.Errorf("discover control-plane endpoints: %w", err)
	}

	prefix, err := clusterprefix.Resolve(ctx, r.APIReader)
	if err != nil {
		return clusterTemplateContext{}, fmt.Errorf("resolve cluster prefix: %w", err)
	}

	return clusterTemplateContext{
		Provider: provider,
		Cluster: clusterTemplateClusterContext{
			Name:            clusterName,
			Namespace:       capiNamespace,
			PodSubnet:       clusterConfig.PodSubnetCIDR,
			ServiceSubnet:   clusterConfig.ServiceSubnetCIDR,
			Domain:          clusterConfig.ClusterDomain,
			Prefix:          prefix,
			MasterEndpoints: endpoints.ClusterMasterEndpoints,
			MasterAddresses: endpoints.APIServerEndpoints,
		},
	}, nil
}

func (r *ClusterReconciler) ensureStaticCluster(ctx context.Context, clusterConfig *clusterConfiguration) error {
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

	logger.V(1).Info("ensured static CAPI cluster resources")
	return nil
}

func (r *ClusterReconciler) createIfNotExists(ctx context.Context, obj *unstructured.Unstructured) error {
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

func (r *ClusterReconciler) readClusterConfiguration(ctx context.Context) (*clusterConfiguration, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name:      clusterConfigSecretName,
		Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf("cluster-configuration secret missing cluster-configuration.yaml key")
	}

	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
	}

	return cfg, nil
}
