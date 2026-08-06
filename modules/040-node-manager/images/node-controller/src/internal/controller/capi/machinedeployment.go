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
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	engineCAPI = "CAPI"
	engineMCM  = "MCM"

	// mdCleanupFinalizer holds the NodeGroup until its MachineDeployments are deleted.
	mdCleanupFinalizer = "node-manager.deckhouse.io/capi-md-cleanup"

	// resyncInterval bounds staleness of rendered MachineClass/MachineDeployment when an
	// input the controller does not watch (e.g. a provider-specific InstanceClass spec)
	// changes. The cloud-provider secret is watched directly for faster reaction.
	resyncInterval = 10 * time.Minute

	// cleanupRetryInterval re-checks a deleted NodeGroup whose MachineDeployments are still
	// terminating. The MachineDeployment Delete event drives the common case; this only backs
	// it up.
	cleanupRetryInterval = 15 * time.Second
)

// The primary object is the unstructured NodeGroup, not the typed one, so that the event, the
// typed value the logic runs on and the raw spec the rendered objects are hashed from all come
// from a single informer. Reading the typed object from one informer and the raw spec from
// another gave two views of the same NodeGroup: under load they disagreed, and a spec hashed
// from the disagreeing half produced a transient MachineTemplate name — an extra node rollout.
func init() {
	register.RegisterController("capi-machine-deployment",
		newUnstructured(deckhousev1.GroupVersion.Group, deckhousev1.GroupVersion.Version, "NodeGroup"),
		&MachineDeploymentReconciler{})
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
	// Create events are also dropped: the only creator of these MachineDeployments is this
	// controller's own SSA apply, and re-running the full render right after creating the
	// object doubles the work of a NodeGroup burst for nothing. A deleted MD is restored
	// via the Delete event; resyncInterval covers anything else.
	mdEventFilter := predicate.And(
		predicate.GenerationChangedPredicate{},
		predicate.Funcs{CreateFunc: func(event.CreateEvent) bool { return false }},
	)
	w.Watches(mcmMD, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup),
		builder.WithPredicates(mdEventFilter))
	w.Watches(&capiv1beta2.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup),
		builder.WithPredicates(mdEventFilter))
	// A change to the cloud-provider secret (provider defaults, instanceClassKind, zones)
	// can change every rendered MachineClass/MachineDeployment, so re-enqueue all NodeGroups.
	w.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.enqueueAllNodeGroups),
		builder.WithPredicates(predicate.NewPredicateFuncs(isCloudProviderSecret)))
	// The InstanceClass is what the MachineClass and the machine template are rendered from,
	// and its checksum names the template — an edit here is exactly what must re-render. Without
	// this watch the change waits for the resync, so the cloud keeps handing out the previous
	// instance type for up to resyncInterval. The source is deferred, not built from a
	// setup-time list: the kind and version come from the provider registration Secret, which
	// may appear only after this pod started.
	w.WatchesRawSource(common.LazyInstanceClassSource(r.Cache,
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			return common.InstanceClassToNodeGroups(ctx, r.Client, obj)
		}),
		predicate.GenerationChangedPredicate{}))
}

// ForPredicates filters NodeGroup events: the rendered MachineDeployments depend only on
// the spec (generation) and annotations (use-mcm, manual-rollout-id) — the engine is derived
// in Reconcile, so status writes by the status controller and finalizer patches must not
// re-enqueue every NodeGroup. resyncInterval still bounds staleness of anything filtered.
func (r *MachineDeploymentReconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)}
}

func mdToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.GetLabels()["node-group"]
	if !ok || ng == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ng}}}
}

func isCloudProviderSecret(obj client.Object) bool {
	return obj.GetNamespace() == cloudProviderSecretNamespace && obj.GetName() == cloudProviderSecretName
}

func (r *MachineDeploymentReconciler) enqueueAllNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(ngList.Items))
	for i := range ngList.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: ngList.Items[i].Name}})
	}
	return reqs
}

func (r *MachineDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	obj := newUnstructured(deckhousev1.GroupVersion.Group, deckhousev1.GroupVersion.Version, "NodeGroup")
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get NodeGroup: %w", err)
	}
	ng := &deckhousev1.NodeGroup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ng); err != nil {
		return ctrl.Result{}, fmt.Errorf("decode NodeGroup %s: %w", req.Name, err)
	}
	rawSpec, _ := obj.Object["spec"].(map[string]interface{})

	if !ng.DeletionTimestamp.IsZero() {
		done, err := r.cleanupMachineDeployments(ctx, ng.Name)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !done {
			// The finalizer holds until the MCM MachineDeployments are really gone: the
			// MachineClass carrying the cloud credentials may only be deleted after them, and
			// once the NodeGroup disappears there is no reconcile left to do it.
			return ctrl.Result{RequeueAfter: cleanupRetryInterval}, nil
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
		// Derive the engine instead of waiting for the status controller to publish
		// status.engine: with the derived value the MachineDeployment is rendered in the
		// first reconcile right after the NodeGroup is created. status.engine, once set,
		// stays the pin (ComputeEngine prefers it).
		cloudProvider, err := r.readCloudProviderTree(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		// Both engines render from the InstanceClass, and the version it is read through decides
		// the checksum that names the template. Guessing one would rename an immutable template
		// and roll every machine in the NodeGroup, so wait instead: the provider secret is
		// watched, and publishing the version re-enqueues this NodeGroup.
		if version, _ := cloudProvider[common.InstanceClassAPIVersionKey].(string); version == "" {
			logger.V(1).Info("skipping: instanceClassAPIVersion is not published yet")
			return ctrl.Result{RequeueAfter: resyncInterval}, nil
		}
		switch derived_status.ComputeEngine(ng, cloudProvider) {
		case engineCAPI:
			if err := r.reconcileCloudMDsRendered(ctx, ng, rawSpec); err != nil {
				return ctrl.Result{}, err
			}
		case engineMCM:
			if err := r.reconcileCloudMCMs(ctx, ng, rawSpec); err != nil {
				return ctrl.Result{}, err
			}
		default:
			logger.V(1).Info("skipping: engine not resolvable", "statusEngine", ng.Status.Engine)
		}
	case deckhousev1.NodeTypeStatic, deckhousev1.NodeTypeCloudStatic:
		if ng.Spec.StaticInstances != nil {
			if err := r.reconcileStaticMDRendered(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: resyncInterval}, nil
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

// cleanupMachineDeployments deletes the CAPI and MCM MachineDeployments belonging to the NodeGroup
// and reports whether the cleanup is finished. A CAPI deployment needs nothing else from us — the
// node drain runs asynchronously under capi/caps-controller-manager finalizers. An MCM one does:
// its MachineClass holds the cloud credentials the deletion itself needs, so the class outlives
// the deployment and the NodeGroup stays finalized until both are gone (see pruneStaleMCMs).
func (r *MachineDeploymentReconciler) cleanupMachineDeployments(ctx context.Context, ngName string) (bool, error) {
	logger := log.FromContext(ctx)

	capiMDs := &unstructured.UnstructuredList{}
	capiMDs.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeploymentList",
	})
	if err := r.Client.List(ctx, capiMDs,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil && client.IgnoreNotFound(err) != nil {
		return false, fmt.Errorf("list CAPI MachineDeployments for NodeGroup %s: %w", ngName, err)
	}
	for i := range capiMDs.Items {
		md := &capiMDs.Items[i]
		if !md.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err := r.Client.Delete(ctx, md); err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("delete MachineDeployment %s: %w", md.GetName(), err)
		}
		logger.V(1).Info("deleted MachineDeployment for removed NodeGroup", "name", md.GetName(), "ng", ngName)
	}

	if err := r.deleteInfraMachineTemplates(ctx, ngName); err != nil {
		return false, err
	}

	cloudProvider, err := r.readCloudProviderTree(ctx)
	if err != nil {
		return false, err
	}
	machineClassKind, _ := cloudProvider["machineClassKind"].(string)
	staleMCMs, err := r.pruneStaleMCMs(ctx, r.APIReader, ngName, machineClassKind, nil, nil)
	if err != nil {
		return false, err
	}

	// The StaticMachineTemplate is named after the NodeGroup and nothing else removes it: helm
	// no longer renders it, set_keep_policy_on_capi_resources marks staticmachinetemplates
	// resource-policy: keep, and an ownerReference is not durable either — caps-controller-manager
	// replaces it with one pointing at the CAPI Cluster, so garbage collection never ties the
	// template back to the NodeGroup. Delete it explicitly, or a NodeGroup recreated under the
	// same name silently adopts the stale labelSelector.
	smt := newUnstructured("infrastructure.cluster.x-k8s.io", "v1alpha1", "StaticMachineTemplate")
	smt.SetName(ngName)
	smt.SetNamespace(common.MachineNamespace)
	if err := r.Client.Delete(ctx, smt); err != nil && !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return false, fmt.Errorf("delete StaticMachineTemplate %s: %w", ngName, err)
	}
	logger.V(1).Info("deleted StaticMachineTemplate for removed NodeGroup", "name", ngName)

	if staleMCMs > 0 {
		logger.V(1).Info("waiting for MCM MachineDeployments to go away before deleting their MachineClasses",
			"ng", ngName, "remaining", staleMCMs)
		return false, nil
	}

	return true, nil
}

// buildStaticMD renders the cluster.x-k8s.io/v1beta2 MachineDeployment for a
// Static/CloudStatic NodeGroup.
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
	capiClusterName                string
	capiMachineTemplateKind        string
	capiMachineTemplateAPIVersion  string
	capiMachineDeploymentSpecPatch string
}

func (r *MachineDeploymentReconciler) readCloudProviderConfig(ctx context.Context) (*cloudProviderConfig, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return &cloudProviderConfig{}, nil
		}
		return nil, fmt.Errorf("get cloud-provider secret: %w", err)
	}

	cfg := &cloudProviderConfig{
		capiClusterName:                string(secret.Data["capiClusterName"]),
		capiMachineTemplateKind:        string(secret.Data["capiMachineTemplateKind"]),
		capiMachineTemplateAPIVersion:  string(secret.Data["capiMachineTemplateAPIVersion"]),
		capiMachineDeploymentSpecPatch: string(secret.Data["capiMachineDeploymentSpecPatch"]),
	}
	if cfg.capiMachineTemplateAPIVersion == "" {
		cfg.capiMachineTemplateAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}
	return cfg, nil
}

func (r *MachineDeploymentReconciler) readClusterUUID(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name: clusterUUIDConfigMapName, Namespace: clusterUUIDConfigMapNS,
	}, cm); err != nil {
		return "", fmt.Errorf("get cluster-uuid configmap: %w", err)
	}
	return cm.Data["cluster-uuid"], nil
}

// readInstancePrefix resolves the cluster prefix via the shared resolver: the
// global ModuleConfig (spec.settings.prefix) takes precedence, falling back to
// the deprecated ClusterConfiguration.cloud.prefix. Kept in one place so the
// webhook, CAPI and the migration controller never diverge.
func (r *MachineDeploymentReconciler) readInstancePrefix(ctx context.Context) (string, error) {
	return clusterprefix.Resolve(ctx, r.Client)
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

// intOrDefault mirrors the helm `| default` these values were rendered with before the
// migration: go templates treat 0 as falsy, so an explicit maxSurgePerZone: 0 (the CRD allows
// it) fell back to the default too. Keeping that matters for maxSurge — 0 together with the
// default maxUnavailable: 0 describes a rollout that can never make progress.
func intOrDefault(ptr *int32, def int) int {
	if ptr != nil && *ptr != 0 {
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
