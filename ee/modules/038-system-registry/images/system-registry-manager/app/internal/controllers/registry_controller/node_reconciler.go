/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	labelTypeKey             = "type"
	labelNodeSecretTypeValue = "node-secret"
	labelHeritageKey         = "heritage"
	labelHeritageValue       = "deckhouse"
)

type NodeReconciler = nodeReconciler

var _ reconcile.Reconciler = &nodeReconciler{}

type nodeReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Namespace string
}

func (r *nodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	nodeWatchPredicate := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			// Only process master nodes
			return nodeObjectIsMaster(e.Object)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			// Only process master nodes
			return nodeObjectIsMaster(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			// Only on master status change
			return nodeObjectIsMaster(e.ObjectOld) != nodeObjectIsMaster(e.ObjectNew)
		},
	}

	secretsWatchPredicate := predicate.NewPredicateFuncs(secretObjectIsNodePKI)

	err := ctrl.NewControllerManagedBy(mgr).
		Named("node-controller").
		For(
			&corev1.Node{},
			builder.WithPredicates(nodeWatchPredicate),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(nodePkiSecretMapFunc),
			builder.WithPredicates(secretsWatchPredicate),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (r *nodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconcile Start",
		"name", req.Name,
		"namespace", req.Namespace,
	)
	defer r.Log.Info("Reconcile Done",
		"name", req.Name,
		"namespace", req.Namespace,
	)

	if req.Namespace != "" {
		r.Log.Info("Fired by supplementary object", "namespace", req.Namespace)
		req.Namespace = ""
	}

	node := &corev1.Node{}
	err := r.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleNodeDelete(ctx, req.Name)
		}

		return ctrl.Result{}, fmt.Errorf("cannot get node: %w", err)
	}

	if hasMasterLabel(node) {
		return r.handleMasterNode(ctx, node)
	} else {
		return r.handleNodeNotMaster(ctx, node)
	}
}

func (r *nodeReconciler) handleMasterNode(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	r.Log.Info("Handle master node", "node", node.Name)

	return ctrl.Result{}, nil
}

func (r *nodeReconciler) handleNodeNotMaster(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := r.deleteNodePKI(ctx, node.Name); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *nodeReconciler) handleNodeDelete(ctx context.Context, name string) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := r.deleteNodePKI(ctx, name); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *nodeReconciler) deleteNodePKI(ctx context.Context, nodeName string) error {
	secretName := fmt.Sprintf("registry-node-%s-pki", nodeName)
	secret := &corev1.Secret{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: r.Namespace}, secret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Already absent
			return nil
		}

		return fmt.Errorf("get node PKI secret error: %w", err)
	}

	err = r.Client.Delete(ctx, secret)

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete node PKI secret error: %w", err)
	}

	return nil
}

func nodePkiSecretMapFunc(ctx context.Context, o client.Object) []reconcile.Request {
	var ret reconcile.Request

	name := o.GetName()
	sub := nodePKISecretRegex.FindStringSubmatch(name)

	if len(sub) < 2 {
		return nil
	}

	ret.Name = sub[1]

	return []reconcile.Request{ret}
}

func secretObjectIsNodePKI(o client.Object) bool {
	labels := o.GetLabels()

	if labels[labelTypeKey] != labelNodeSecretTypeValue {
		return false
	}

	if labels[labelHeritageKey] != labelHeritageValue {
		return false
	}

	return nodePKISecretRegex.MatchString(o.GetName())
}

func nodeObjectIsMaster(object client.Object) bool {
	if object == nil {
		return false
	}

	labels := object.GetLabels()
	if labels == nil {
		return false
	}

	_, hasMasterLabel := labels["node-role.kubernetes.io/master"]

	return hasMasterLabel
}
