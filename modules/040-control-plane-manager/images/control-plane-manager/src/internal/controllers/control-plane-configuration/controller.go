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
	"control-plane-manager/internal/operations"
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
}

func Register(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
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

// getNodeControlPlaneLabelPredicate triggers only when Node labels change
// Ignores updates to status, capacity, etc.
func getNodeControlPlaneLabelPredicate() predicate.Predicate {
	hasLabel := func(o client.Object) bool {
		_, has := o.GetLabels()[constants.ControlPlaneNodeLabelKey]
		return has
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasLabel(e.Object)
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
			return hasLabel(e.ObjectNew) || hasLabel(e.ObjectOld)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return hasLabel(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// mapSecretToControlPlaneNodes enqueues reconcile for every master node (secret change affects all ControlPlaneNodes).
func (r *Reconciler) mapSecretToControlPlaneNodes(ctx context.Context, _ client.Object) []reconcile.Request {
	nodes, err := r.getControlPlaneNodes(ctx)
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

// getControlPlaneNodes helper function to get all nodes with control plane label.
func (r *Reconciler) getControlPlaneNodes(ctx context.Context) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	err := r.client.List(ctx, nodeList, client.MatchingLabels{
		constants.ControlPlaneNodeLabelKey: "",
	})
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
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
	if _, hasLabel := node.Labels[constants.ControlPlaneNodeLabelKey]; !hasLabel {
		// No longer a master — remove ControlPlaneNode
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

	desiredCPN, err := buildDesiredControlPlaneNode(nodeName, cpmSecret, pkiSecret)
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
func buildDesiredControlPlaneNode(nodeName string, cpmSecret *corev1.Secret, pkiSecret *corev1.Secret) (*controlplanev1alpha1.ControlPlaneNode, error) {
	pkiChecksum, err := checksum.PKIChecksum(pkiSecret.Data)
	if err != nil {
		return nil, err
	}

	components := []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"}
	checksums := make(map[string]string)
	for _, component := range components {
		componentChecksum, err := checksum.ComponentChecksum(cpmSecret.Data, component)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", component, err)
		}
		checksums[component] = componentChecksum
	}
	hotReloadChecksum, err := checksum.HotReloadChecksum(cpmSecret.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hot reload checksum: %w", err)
	}
	// temporary for testing
	err = operations.SyncSecretToTmp(cpmSecret, "/tmp/control-plane-manager-config")
	if err != nil {
		return nil, fmt.Errorf("failed to sync secret to tmp: %w", err)
	}

	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: ctrl.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: nodeName,
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneNodeSpec{
			PKIChecksum:       pkiChecksum,
			ConfigVersion:     fmt.Sprintf("%s.%s", cpmSecret.ResourceVersion, pkiSecret.ResourceVersion),
			HotReloadChecksum: hotReloadChecksum,
			Components: controlplanev1alpha1.ComponentChecksums{
				Etcd:                  controlplanev1alpha1.ComponentChecksum{Checksum: checksums["etcd"]},
				KubeAPIServer:         controlplanev1alpha1.ComponentChecksum{Checksum: checksums["kube-apiserver"]},
				KubeControllerManager: controlplanev1alpha1.ComponentChecksum{Checksum: checksums["kube-controller-manager"]},
				KubeScheduler:         controlplanev1alpha1.ComponentChecksum{Checksum: checksums["kube-scheduler"]},
			},
		},
	}, nil
}
