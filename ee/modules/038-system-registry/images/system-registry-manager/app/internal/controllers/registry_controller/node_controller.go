/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"

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
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	labelTypeKey             = "type"
	labelNodeSecretTypeValue = "node-secret"
	labelHeritageKey         = "heritage"
	labelHeritageValue       = "deckhouse"
	labelNodeIsMasterKey     = "node-role.kubernetes.io/master"
)

type NodeController = nodeController

var _ reconcile.Reconciler = &nodeController{}

type nodeController struct {
	Client    client.Client
	Namespace string

	reprocessCh chan event.TypedGenericEvent[reconcile.Request]
}

var nodeReprocessAllRequest = reconcile.Request{
	NamespacedName: types.NamespacedName{
		Namespace: "--reprocess-all-nodes--",
		Name:      "--reprocess-all-nodes--",
	},
}

func (r *nodeController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	r.reprocessCh = make(chan event.TypedGenericEvent[reconcile.Request])

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
		WatchesRawSource(r.reprocessChannelSource()).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (r *nodeController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if req == nodeReprocessAllRequest {
		return r.handleReprocessAll(ctx)
	}

	if req.Namespace != "" {
		log.Info("Fired by supplementary object", "namespace", req.Namespace)
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

func (r *nodeController) handleReprocessAll(ctx context.Context) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("ReprocessAll: Start")
	defer log.Info("ReprocessAll: Done")

	// Will trigger reprocess for all master nodes
	opts := client.MatchingLabels{
		labelNodeIsMasterKey: "",
	}

	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes, &opts); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		req := reconcile.Request{}
		req.Name = node.Name
		req.Namespace = node.Namespace

		if err := r.scheduleReconcileForNode(ctx, req); err != nil {
			// It currently only when ctx done
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *nodeController) scheduleReconcileForNode(ctx context.Context, req reconcile.Request) error {
	evt := event.TypedGenericEvent[reconcile.Request]{Object: req}

	select {
	case r.reprocessCh <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *nodeController) handleMasterNode(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Handle master node", "node", node.Name)

	return ctrl.Result{}, nil
}

func (r *nodeController) handleNodeNotMaster(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := r.deleteNodePKI(ctx, node.Name); err != nil {
		return ctrl.Result{}, err
	}

	/*
		Here we may stop static pods, but it will be a race with k8s scheduler
		So NOOP
	*/

	return ctrl.Result{}, nil
}

func (r *nodeController) handleNodeDelete(ctx context.Context, name string) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := r.deleteNodePKI(ctx, name); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *nodeController) deleteNodePKI(ctx context.Context, nodeName string) error {
	log := ctrl.LoggerFrom(ctx)

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
	} else {
		log.Info("Deleted node PKI", "node", nodeName, "name", secret.Name, "namespace", secret.Namespace)
	}

	return nil
}

func (r *nodeController) reprocessChannelSource() source.Source {
	return source.Channel(r.reprocessCh, handler.TypedEnqueueRequestsFromMapFunc(
		func(ctx context.Context, req reconcile.Request) []reconcile.Request {
			return []reconcile.Request{req}
		},
	))
}

func (r *nodeController) ReprocessAllNodes(ctx context.Context) error {
	evt := event.TypedGenericEvent[reconcile.Request]{Object: nodeReprocessAllRequest}

	select {
	case r.reprocessCh <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
