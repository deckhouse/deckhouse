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
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("capi-cluster-resources", &corev1.Secret{}, &ClusterReconciler{})
}

type ClusterReconciler struct {
	BaseWithReader
}

const (
	// InfrastructureCluster GVKs are provider registration data, so they cannot be watched
	// statically without hardcoding providers. Periodic reconciliation repairs deleted outputs.
	clusterRepairInterval         = 5 * time.Minute
	capiClusterCredentialsLabel   = "node-manager.deckhouse.io/capi-cluster-credentials"
	capiClusterCredentialsManaged = "true"
)

func (r *ClusterReconciler) MaxConcurrentReconciles() int {
	return 1
}

var clusterReconcileRequest = []reconcile.Request{{NamespacedName: types.NamespacedName{
	Name:      cloudProviderSecretName,
	Namespace: cloudProviderSecretNamespace,
}}}

func (r *ClusterReconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{
		predicate.NewPredicateFuncs(cloudprovider.IsInputSecret),
		predicate.ResourceVersionChangedPredicate{},
	}
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
	w.Watches(&corev1.Pod{}, enqueue, builder.WithPredicates(apiServerPodEndpointChanged()))
	w.Watches(&discoveryv1.EndpointSlice{}, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == "default" && obj.GetName() == "kubernetes"
		}),
		predicate.ResourceVersionChangedPredicate{},
	))

	// Cluster templates may use the global cluster prefix to name provider resources.
	moduleConfig := newUnstructured("deckhouse.io", "v1alpha1", "ModuleConfig")
	w.Watches(moduleConfig, enqueue, builder.WithPredicates(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetName() == clusterprefix.GlobalModuleConfigName
		}),
		predicate.ResourceVersionChangedPredicate{},
	))
}

func apiServerPodEndpointChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return isAPIServerPod(event.Object)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return isAPIServerPod(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldPod, oldOK := event.ObjectOld.(*corev1.Pod)
			newPod, newOK := event.ObjectNew.(*corev1.Pod)
			if !oldOK || !newOK {
				return false
			}
			oldRelevant := isAPIServerPod(oldPod)
			newRelevant := isAPIServerPod(newPod)
			if oldRelevant != newRelevant {
				return true
			}
			if !newRelevant {
				return false
			}
			return oldPod.Status.PodIP != newPod.Status.PodIP || common.PodReady(oldPod) != common.PodReady(newPod)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func isAPIServerPod(object client.Object) bool {
	return object.GetNamespace() == common.KubeSystemNamespace && common.IsAPIServerPod(object)
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	clusterConfig, err := common.ReadClusterConfiguration(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	staticErr := r.ensureStaticCluster(ctx, clusterConfig)
	cloudErr := r.ensureCloudClusters(ctx, clusterConfig)
	if err := errors.Join(staticErr, cloudErr); err != nil {
		// A returned error is already requeued with backoff, and controller-runtime drops
		// RequeueAfter when one is present. Returning both would read as a repair interval
		// that does not exist.
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: clusterRepairInterval}, nil
}

// ensureCloudClusters builds the CAPI scaffolding of every registered provider: a NodeGroup that
// runs on one needs its Cluster, and the catalog is the only place that knows they exist.
func (r *ClusterReconciler) ensureCloudClusters(ctx context.Context, clusterConfig common.ClusterConfiguration) error {
	catalog, err := cloudprovider.GetCatalog(ctx, r.Client)
	if err != nil {
		return err
	}

	source := cloudprovider.Source{Reader: r.Client}
	var errs []error
	var rendered []providerClusterResources
	// Whether the desired set below names every provider there is. One that could not be read
	// leaves it incomplete, and cleaning up against an incomplete set deletes live credentials.
	complete := true

	for _, registration := range catalog.All() {
		if registration.IsStatic() || !registration.HasCAPI() {
			continue
		}
		if err := registration.ValidateCore(); err != nil {
			errs = append(errs, err)
			complete = false
			continue
		}
		if err := registration.ValidateCAPI(); err != nil {
			errs = append(errs, err)
			complete = false
			continue
		}

		// The common scaffolding is rendered from the registration and the cluster configuration
		// alone, so it goes out first: a cluster whose UUID ConfigMap or prefix cannot be read yet
		// still gets its Cluster, MachineHealthCheck and control plane.
		errs = append(errs, r.ensureCommonCloudClusterResources(ctx, registration, clusterConfig))

		provider, err := source.LoadWithClusterConfiguration(ctx, registration, clusterConfig)
		if err != nil {
			errs = append(errs, err)
			complete = false
			continue
		}
		resources, err := r.renderProviderClusterResources(ctx, source, provider)
		if err != nil {
			errs = append(errs, err)
			complete = false
			continue
		}
		rendered = append(rendered, resources)
	}

	// Cleanup runs before the applies, not after: a failed apply must not leave a Secret with the
	// old provider's live cloud credentials.
	if complete && len(rendered) > 0 {
		desired := make(map[string]bool, len(rendered))
		for _, resources := range rendered {
			if resources.credentials != nil {
				desired[resources.credentials.GetName()] = true
			}
		}
		errs = append(errs, r.removeStaleProviderCredentials(ctx, desired))
	}

	for _, resources := range rendered {
		errs = append(errs, r.applyProviderClusterResources(ctx, resources))
	}
	return errors.Join(errs...)
}

type providerClusterResources struct {
	credentials    *unstructured.Unstructured
	infrastructure *unstructured.Unstructured
}

func (r *ClusterReconciler) renderProviderClusterResources(
	ctx context.Context,
	source cloudprovider.Source,
	provider cloudprovider.Provider,
) (providerClusterResources, error) {
	inputs, err := source.LoadCAPIClusterInputs(ctx, provider)
	if err != nil {
		return providerClusterResources{}, err
	}
	renderData := provider.RenderData()
	renderData.ControlPlane = r.controlPlaneEndpoints(ctx)
	credentials, err := renderProviderCredentials(provider, inputs, renderData)
	if err != nil {
		return providerClusterResources{}, err
	}
	infrastructure, err := renderProviderInfrastructure(provider, inputs, renderData)
	if err != nil {
		return providerClusterResources{}, err
	}
	return providerClusterResources{credentials: credentials, infrastructure: infrastructure}, nil
}

func (r *ClusterReconciler) applyProviderClusterResources(ctx context.Context, resources providerClusterResources) error {
	var errs []error
	if resources.credentials != nil {
		if err := r.applyClusterObject(ctx, resources.credentials); err != nil {
			errs = append(errs, fmt.Errorf("apply credentials Secret %s: %w", resources.credentials.GetName(), err))
		}
	}
	if err := r.applyClusterObject(ctx, resources.infrastructure); err != nil {
		errs = append(errs, fmt.Errorf("apply provider infrastructure %s %s: %w",
			resources.infrastructure.GetKind(), resources.infrastructure.GetName(), err))
	}
	return errors.Join(errs...)
}

func (r *ClusterReconciler) ensureCommonCloudClusterResources(
	ctx context.Context,
	registration cloudprovider.Registration,
	clusterConfig common.ClusterConfiguration,
) error {
	logger := log.FromContext(ctx)
	clusterName := registration.CAPIClusterName
	clusterKind := registration.CAPIClusterKind
	infraGV, err := schema.ParseGroupVersion(registration.CAPIClusterAPIVersion)
	if err != nil {
		return fmt.Errorf("parse capiClusterAPIVersion %q: %w", registration.CAPIClusterAPIVersion, err)
	}

	var ensureErrors []error
	controlPlane := deckhouseControlPlane(clusterName)
	if err := r.applyClusterObject(ctx, controlPlane); err != nil {
		ensureErrors = append(ensureErrors, fmt.Errorf("apply DeckhouseControlPlane %s: %w", controlPlane.GetName(), err))
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
				"apiGroup": infraGV.Group,
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
		ensureErrors = append(ensureErrors, fmt.Errorf("create Cluster: %w", err))
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
		ensureErrors = append(ensureErrors, fmt.Errorf("create MachineHealthCheck: %w", err))
	}

	result := errors.Join(ensureErrors...)
	if result == nil {
		logger.V(1).Info("ensured common cloud CAPI cluster resources", "cluster", clusterName)
	}
	return result
}

func renderProviderCredentials(
	provider cloudprovider.Provider,
	inputs cloudprovider.CAPIClusterInputs,
	data cloudprovider.RenderData,
) (*unstructured.Unstructured, error) {
	if inputs.Credentials == nil {
		return nil, nil
	}
	secret, err := inputs.Credentials.Render(data)
	if err != nil {
		return nil, fmt.Errorf("cloud provider %s: %w", provider.Registration.Type, err)
	}
	labels := secret.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[capiClusterCredentialsLabel] = capiClusterCredentialsManaged
	secret.SetLabels(labels)

	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, fmt.Errorf("convert credentials Secret %s: %w", secret.Name, err)
	}
	unstructuredSecret := &unstructured.Unstructured{Object: object}
	unstructuredSecret.SetAPIVersion("v1")
	unstructuredSecret.SetKind("Secret")
	return unstructuredSecret, nil
}

func renderProviderInfrastructure(
	provider cloudprovider.Provider,
	inputs cloudprovider.CAPIClusterInputs,
	data cloudprovider.RenderData,
) (*unstructured.Unstructured, error) {
	object, err := inputs.Cluster.Render(data)
	if err != nil {
		return nil, fmt.Errorf("cloud provider %s: %w", provider.Registration.Type, err)
	}
	return object, nil
}

// removeStaleProviderCredentials deletes the managed credentials Secrets other than the one the
// provider template renders. An empty desired name means the provider ships no credentials.yaml at
// all, and then every managed Secret is stale: the label is set only by renderProviderCredentials,
// and the provider template Secret is one Helm-rendered object, so a missing credentials.yaml is a
// deliberate removal rather than a half-written Secret. Callers reach this only after both
// templates rendered, so a render failure never looks like an empty name.
func (r *ClusterReconciler) removeStaleProviderCredentials(ctx context.Context, desired map[string]bool) error {
	secrets := &corev1.SecretList{}
	if err := r.APIReader.List(
		ctx,
		secrets,
		client.InNamespace(capiNamespace),
		client.MatchingLabels{capiClusterCredentialsLabel: capiClusterCredentialsManaged},
	); err != nil {
		return fmt.Errorf("list provider credentials Secrets: %w", err)
	}

	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if desired[secret.Name] {
			continue
		}
		if err := r.Client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale provider credentials Secret %s: %w", secret.Name, err)
		}
	}
	return nil
}

func (r *ClusterReconciler) applyClusterObject(ctx context.Context, object *unstructured.Unstructured) error {
	prepareClusterTemplateObject(object)

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(object.GroupVersionKind())
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(object), current); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("read live object: %w", err)
		}
	} else if endpoint, found, err := unstructured.NestedMap(current.Object, "spec", "controlPlaneEndpoint"); err != nil {
		return fmt.Errorf("read live controlPlaneEndpoint: %w", err)
	} else if found {
		// The live endpoint is kept only when the template rendered none: most providers fill it in
		// via their own controller. OpenStack renders it from the apiserver addresses this
		// controller watches, and there the rendered value must win — a stale one would pin the
		// cluster to a master that is gone.
		_, rendered, err := unstructured.NestedMap(object.Object, "spec", "controlPlaneEndpoint")
		if err != nil {
			return fmt.Errorf("read rendered controlPlaneEndpoint: %w", err)
		}
		if !rendered {
			if err := ensureObjectSpecMap(object); err != nil {
				return err
			}
			if err := unstructured.SetNestedMap(object.Object, endpoint, "spec", "controlPlaneEndpoint"); err != nil {
				return fmt.Errorf("preserve live controlPlaneEndpoint: %w", err)
			}
		}
	}

	if err := r.Client.Patch(
		ctx,
		object,
		client.Apply,
		client.FieldOwner("node-controller"),
		client.ForceOwnership,
	); err != nil {
		return err
	}

	return nil
}

func ensureObjectSpecMap(object *unstructured.Unstructured) error {
	value, found, err := unstructured.NestedFieldNoCopy(object.Object, "spec")
	if err != nil {
		return fmt.Errorf("read rendered spec: %w", err)
	}
	if !found || value == nil {
		if err := unstructured.SetNestedMap(object.Object, map[string]any{}, "spec"); err != nil {
			return fmt.Errorf("initialize rendered spec: %w", err)
		}
		return nil
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("rendered spec is %T, want an object", value)
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

func (r *ClusterReconciler) controlPlaneEndpoints(ctx context.Context) []cloudprovider.ControlPlaneEndpoint {
	endpoints, err := (&bashiblecontext.Service{
		Client: r.Client,
		Reader: r.APIReader,
	}).ReadEndpoints(ctx)
	if err != nil {
		log.FromContext(ctx).V(1).Info("control-plane endpoints are not available", "error", err)
		return nil
	}

	result := make([]cloudprovider.ControlPlaneEndpoint, 0, len(endpoints.ClusterMasterEndpoints))
	for _, endpoint := range endpoints.ClusterMasterEndpoints {
		host, _ := endpoint["address"].(string)
		port, _ := endpoint["kubeApiPort"].(int)
		if host == "" || port == 0 {
			continue
		}
		result = append(result, cloudprovider.ControlPlaneEndpoint{Host: host, Port: port})
	}
	return result
}

func (r *ClusterReconciler) ensureStaticCluster(ctx context.Context, clusterConfig common.ClusterConfiguration) error {
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
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
