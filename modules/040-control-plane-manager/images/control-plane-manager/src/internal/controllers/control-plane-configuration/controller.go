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

package controlplaneconfiguration

import (
	"context"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
	"fmt"
	"log/slog"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"github.com/deckhouse/deckhouse/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"golang.org/x/time/rate"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client client.Client
	log    *log.Logger
}

func Register(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		log:    log.Default().With(slog.String("controller", constants.CpcControllerName)),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpcControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneNode{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(getControlPlaneNodeResourcePredicate()),
		).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretToControlPlaneNodes),
			builder.WithPredicates(getSecretPredicate()),
		).
		Watches(&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.mapNodeToControlPlaneNode),
			builder.WithPredicates(getNodeControlPlaneLabelPredicate()),
		).
		Complete(r)
}

// getSecretPredicate checks if the secret is d8-control-plane-manager-config or d8-pki.
func getSecretPredicate() predicate.Predicate {
	isTarget := func(o client.Object) bool {
		return (o.GetName() == constants.ControlPlaneManagerConfigSecretName || o.GetName() == constants.PkiSecretName) &&
			o.GetNamespace() == constants.KubeSystemNamespace
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isTarget(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isTarget(e.ObjectNew) },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// getControlPlaneNodeResourcePredicate triggers on any create/update/delete of ControlPlaneNode CR.
func getControlPlaneNodeResourcePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// isControlPlaneOrArbiter returns true if the node has the control-plane or etcd-arbiter label.
func isControlPlaneOrArbiter(o client.Object) bool {
	labels := o.GetLabels()
	_, cp := labels[constants.ControlPlaneNodeLabelKey]
	_, arb := labels[constants.EtcdArbiterNodeLabelKey]
	return cp || arb
}

// getNodeControlPlaneLabelPredicate triggers only when Node labels change for nodes that are control-plane or etcd-arbiter.
func getNodeControlPlaneLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isControlPlaneOrArbiter(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, okOld := e.ObjectOld.(*corev1.Node)
			newNode, okNew := e.ObjectNew.(*corev1.Node)
			if !okOld || !okNew {
				return false
			}
			if equality.Semantic.DeepEqual(oldNode.Labels, newNode.Labels) {
				return false
			}
			return isControlPlaneOrArbiter(e.ObjectNew) || isControlPlaneOrArbiter(e.ObjectOld)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isControlPlaneOrArbiter(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// mapSecretToControlPlaneNodes enqueues reconcile for every control-plane and arbiter node.
func (r *Reconciler) mapSecretToControlPlaneNodes(ctx context.Context, _ client.Object) []reconcile.Request {
	nodes, err := r.getControlPlaneAndArbiterNodes(ctx)
	if err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(nodes))
	for _, node := range nodes {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKey{Name: node.Name}})
	}
	return reqs
}

// mapNodeToControlPlaneNode enqueues reconcile for the ControlPlaneNode matching the master Node (same name).
func (r *Reconciler) mapNodeToControlPlaneNode(ctx context.Context, object client.Object) []reconcile.Request {
	node, ok := object.(*corev1.Node)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: node.Name}}}
}

// getSecret helper function to get secret from kube-system namespace by name.
func (r *Reconciler) getSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: constants.KubeSystemNamespace,
	}, secret)

	if err != nil {
		return nil, err
	}

	return secret, nil
}

// getControlPlaneAndArbiterNodes returns all nodes that are control-plane or etcd-arbiter.
func (r *Reconciler) getControlPlaneAndArbiterNodes(ctx context.Context) ([]corev1.Node, error) {
	cpList := &corev1.NodeList{}
	if err := r.client.List(ctx, cpList, client.MatchingLabels{
		constants.ControlPlaneNodeLabelKey: "",
	}); err != nil {
		return nil, err
	}

	arbList := &corev1.NodeList{}
	if err := r.client.List(ctx, arbList, client.MatchingLabels{
		constants.EtcdArbiterNodeLabelKey: "",
	}); err != nil {
		return nil, err
	}

	// Merge/dedup by name (control-plane and etcd-arbiter labels are mutually exclusive)
	seen := make(map[string]struct{}, len(cpList.Items))
	result := make([]corev1.Node, 0, len(cpList.Items)+len(arbList.Items))
	for _, n := range cpList.Items {
		seen[n.Name] = struct{}{}
		result = append(result, n)
	}
	for _, n := range arbList.Items {
		if _, exists := seen[n.Name]; !exists {
			result = append(result, n)
		}
	}
	return result, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeName := req.Name
	log.Info("Reconcile started for ControlPlaneNode", slog.String("node", nodeName))

	node := &corev1.Node{}
	err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Node gone — remove ControlPlaneNode if it exists
			if err := r.deleteControlPlaneNodeIfExists(ctx, nodeName); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	_, isControlPlane := node.Labels[constants.ControlPlaneNodeLabelKey]
	_, isArbiter := node.Labels[constants.EtcdArbiterNodeLabelKey]
	if !isControlPlane && !isArbiter {
		if err := r.deleteControlPlaneNodeIfExists(ctx, nodeName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	cpmSecret, err := r.getSecret(ctx, constants.ControlPlaneManagerConfigSecretName)
	if err != nil {
		log.Error("Error occurred while getting secret",
			slog.String("secret", constants.ControlPlaneManagerConfigSecretName),
			slog.String("namespace", constants.KubeSystemNamespace),
			log.Err(err),
		)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	pkiSecret, err := r.getSecret(ctx, constants.PkiSecretName)
	if err != nil {
		log.Error("Error occurred while getting secret",
			slog.String("secret", constants.PkiSecretName),
			slog.String("namespace", constants.KubeSystemNamespace),
			log.Err(err),
		)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	desiredCPN, err := buildDesiredControlPlaneNode(nodeName, cpmSecret, pkiSecret, isArbiter)
	if err != nil {
		log.Error("Error occurred while building desired ControlPlaneNode", slog.String("node", nodeName), log.Err(err))
		return reconcile.Result{}, err
	}
	if err := r.applyControlPlaneNode(ctx, desiredCPN); err != nil {
		log.Error("Error occurred while reconciling ControlPlaneNode", slog.String("node", nodeName), log.Err(err))
		return reconcile.Result{}, err
	}

	log.Info("Reconcile completed for ControlPlaneNode", slog.String("node", nodeName))
	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// deleteControlPlaneNodeIfExists deletes ControlPlaneNode if it exists immediately.
func (r *Reconciler) deleteControlPlaneNodeIfExists(ctx context.Context, name string) error {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, cpn)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	log.Info("Deleting orphaned ControlPlaneNode", slog.String("name", name))
	return client.IgnoreNotFound(r.client.Delete(ctx, cpn))
}

// applyControlPlaneNode applies desired ControlPlaneNode spec to the current ControlPlaneNode using patch.
func (r *Reconciler) applyControlPlaneNode(ctx context.Context, desired *controlplanev1alpha1.ControlPlaneNode) error {
	current := &controlplanev1alpha1.ControlPlaneNode{}
	key := client.ObjectKey{Name: desired.Name}
	err := r.client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		log.Info("ControlPlaneNode not found, creating", slog.String("node", desired.Name))
		return r.client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(desired.Spec, current.Spec) {
		log.Info("ControlPlaneNode spec differs from desired, updating", slog.String("node", desired.Name))
		patch := client.MergeFrom(current.DeepCopy())
		current.Spec = desired.Spec
		return r.client.Patch(ctx, current, patch)
	}
	return nil
}

// buildDesiredControlPlaneNode builds desired ControlPlaneNode spec from d8-control-plane-manager-config and d8-pki secrets.
// For etcd-arbiter nodes only Etcd and CA checksums are populated.
func buildDesiredControlPlaneNode(nodeName string, cpmSecret *corev1.Secret, pkiSecret *corev1.Secret, isArbiter bool) (*controlplanev1alpha1.ControlPlaneNode, error) {
	caChecksum, err := checksum.PKIChecksum(pkiSecret.Data)
	if err != nil {
		return nil, err
	}

	var components []string
	if isArbiter {
		components = []string{"etcd"}
	} else {
		components = []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"}
	}

	configChecksums := make(map[string]string)
	pkiChecksums := make(map[string]string)
	for _, component := range components {
		cs, err := checksum.ComponentChecksum(cpmSecret.Data, component)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", component, err)
		}
		configChecksums[component] = cs

		pkiCS, err := checksum.ComponentPKIChecksum(cpmSecret.Data, component)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate pki checksum for %s: %w", component, err)
		}
		pkiChecksums[component] = pkiCS
	}

	spec := controlplanev1alpha1.ControlPlaneNodeSpec{
		CAChecksum: caChecksum,
		Components: controlplanev1alpha1.ComponentsSpec{
			Etcd: controlplanev1alpha1.ComponentSpec{
				Checksums: controlplanev1alpha1.Checksums{
					Config: configChecksums["etcd"],
					PKI:    pkiChecksums["etcd"],
				},
			},
		},
	}

	if !isArbiter {
		hotReloadChecksum := checksum.HotReloadChecksum(cpmSecret.Data)
		spec.HotReloadChecksum = hotReloadChecksum
		spec.Components.KubeAPIServer = controlplanev1alpha1.ComponentSpec{
			Checksums: controlplanev1alpha1.Checksums{
				Config: configChecksums["kube-apiserver"],
				PKI:    pkiChecksums["kube-apiserver"],
			},
		}
		spec.Components.KubeControllerManager = controlplanev1alpha1.ComponentSpec{
			Checksums: controlplanev1alpha1.Checksums{
				Config: configChecksums["kube-controller-manager"],
			},
		}
		spec.Components.KubeScheduler = controlplanev1alpha1.ComponentSpec{
			Checksums: controlplanev1alpha1.Checksums{
				Config: configChecksums["kube-scheduler"],
			},
		}
	}

	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: ctrl.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: nodeName,
				constants.HeritageLabelKey:             constants.HeritageLabelValue,
			},
		},
		Spec: spec,
	}, nil
}
