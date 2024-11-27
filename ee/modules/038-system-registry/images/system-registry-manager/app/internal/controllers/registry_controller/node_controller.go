/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"
	"regexp"

	"embeded-registry-manager/internal/state"
	"embeded-registry-manager/internal/utils/pki"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var (
	nodePKISecretRegex        = regexp.MustCompile(`^registry-node-(.*)-pki$`)
	masterNodesMatchingLabels = client.MatchingLabels{
		state.LabelNodeIsMasterKey: "",
	}
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

	controllerName := "node-controller"

	// TODO
	// registryAddress := os.Getenv("REGISTRY_ADDRESS")
	// registryPath := os.Getenv("REGISTRY_PATH")
	// imageDockerAuth := os.Getenv("IMAGE_DOCKER_AUTH")
	// imageDockerDistribution := os.Getenv("IMAGE_DOCKER_DISTRIBUTION")

	// Set up the field indexer to index Pods by spec.nodeName
	err := mgr.GetFieldIndexer().
		IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
			pod := obj.(*corev1.Pod)
			return []string{pod.Spec.NodeName}
		})

	if err != nil {
		return fmt.Errorf("failed to set up index on spec.nodeName: %w", err)
	}

	nodePredicate := predicate.Funcs{
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

	secretsPredicate := predicate.NewPredicateFuncs(secretObjectIsNodePKI)

	moduleConfig := state.GetModuleConfigObject()
	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == state.RegistryModuleName
	})

	globalSecretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != r.Namespace {
			return false
		}

		name := obj.GetName()

		return name == state.PKISecretName || name == state.UserROSecretName || name == state.UserRWSecretName
	})

	reprocessAllHandler := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{nodeReprocessAllRequest}
	})

	err = ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(
			&corev1.Node{},
			builder.WithPredicates(nodePredicate),
			builder.OnlyMetadata,
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				name := obj.GetName()
				sub := nodePKISecretRegex.FindStringSubmatch(name)

				if len(sub) < 2 {
					return nil
				}

				var ret reconcile.Request
				ret.Name = sub[1]

				log := ctrl.LoggerFrom(ctx)

				log.Info(
					"Node PKI secret changed, will trigger reconcile",
					"secret", obj.GetName(),
					"namespace", obj.GetNamespace(),
					"node", ret.Name,
					"controller", controllerName,
				)

				return []reconcile.Request{ret}
			}),
			builder.WithPredicates(secretsPredicate),
		).
		WatchesRawSource(r.reprocessChannelSource()).
		Watches(
			&moduleConfig,
			reprocessAllHandler,
			builder.WithPredicates(moduleConfigPredicate),
		).
		Watches(
			&corev1.Secret{},
			reprocessAllHandler,
			builder.WithPredicates(globalSecretsPredicate),
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

func (nc *nodeController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req == nodeReprocessAllRequest {
		return nc.handleReprocessAll(ctx)
	}

	if req.Namespace != "" {
		req.Namespace = ""
	}

	node := &metav1.PartialObjectMetadata{}
	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	})

	err := nc.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nc.handleNodeDelete(ctx, req.Name)
		}

		return ctrl.Result{}, fmt.Errorf("cannot get node: %w", err)
	}

	if hasMasterLabel(node) {
		return nc.handleMasterNode(ctx, node)
	} else {
		return nc.handleNodeNotMaster(ctx, node)
	}
}

func (nc *nodeController) handleReprocessAll(ctx context.Context) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("All nodes will be reprocessed")

	// Will trigger reprocess for all master nodes
	nodes, err := nc.getAllMasterNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, node := range nodes.Items {
		req := reconcile.Request{}
		req.Name = node.Name
		req.Namespace = node.Namespace

		if err := nc.triggerReconcileForNode(ctx, req); err != nil {
			// It currently only when ctx done
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (nc *nodeController) triggerReconcileForNode(ctx context.Context, req reconcile.Request) error {
	evt := event.TypedGenericEvent[reconcile.Request]{Object: req}

	select {
	case nc.reprocessCh <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (nc *nodeController) handleMasterNode(ctx context.Context, node *metav1.PartialObjectMetadata) (result ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx)

	config, err := state.LoadModuleConfig(ctx, nc.Client)
	if err != nil {
		err = fmt.Errorf("cannot load module config: %w", err)
		return
	}

	if !config.Enabled {
		return
	}

	userRO, err := nc.loadUserSecret(ctx, state.UserROSecretName)
	if err != nil {
		err = fmt.Errorf("cannot load RO user: %w", err)
		return
	}

	userRW, err := nc.loadUserSecret(ctx, state.UserRWSecretName)
	if err != nil {
		err = fmt.Errorf("cannot load RW user: %w", err)
		return
	}

	pkiState, err := nc.loadGlobalPKI(ctx)
	if err != nil {
		err = fmt.Errorf("cannot load global PKI: %w", err)
		return
	}

	// TODO
	_ = userRO
	_ = userRW
	_ = pkiState

	isFirst, err := nc.isFirstMasterNode(ctx, node)
	if err != nil {
		return
	}

	log.Info("Processing master node", "node", node.Name, "first", isFirst)

	return
}

func (nc *nodeController) loadUserSecret(ctx context.Context, name string) (ret state.User, err error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return
	}

	ret.UserName = string(secret.Data["name"])
	ret.Password = string(secret.Data["password"])
	ret.HashedPassword = string(secret.Data["passwordHash"])

	if !ret.IsValid() {
		err = fmt.Errorf("user data is invalid")
		return
	}

	if !ret.IsPasswordHashValid() {
		err = fmt.Errorf("password hash not corresponding to password")
		return
	}

	return
}

func (nc *nodeController) loadGlobalPKI(ctx context.Context) (ret state.PKIState, err error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.PKISecretName,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return
	}

	caPKI, err := state.DecodeCertKeyFromSecret(
		state.CACertSecretField,
		state.CAKeySecretField,
		&secret,
	)

	if err != nil {
		err = fmt.Errorf("cannot decode CA PKI: %w", err)
		return
	}
	ret.CA = &caPKI

	tokenPKI, err := state.DecodeCertKeyFromSecret(
		state.TokenCertSecretField,
		state.TokenKeySecretField,
		&secret,
	)

	if err != nil {
		err = fmt.Errorf("cannot decode Token PKI: %w", err)
		return
	}
	ret.Token = &tokenPKI

	if err = pki.ValidateCertWithCAChain(ret.Token.Cert, ret.CA.Cert); err != nil {
		err = fmt.Errorf("certificate validation error for Token: %w", err)
		return
	}

	return
}

func (nc *nodeController) isFirstMasterNode(ctx context.Context, node *metav1.PartialObjectMetadata) (bool, error) {
	nodes, err := nc.getAllMasterNodes(ctx)
	if err != nil {
		return false, err
	}

	for _, item := range nodes.Items {
		if item.Name == node.Name {
			continue
		}

		if item.CreationTimestamp.Before(&node.CreationTimestamp) {
			return false, nil
		}
	}

	return true, nil
}

func (nc *nodeController) getAllMasterNodes(ctx context.Context) (nodes *metav1.PartialObjectMetadataList, err error) {
	nodes = &metav1.PartialObjectMetadataList{}
	nodes.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	})

	if err = nc.Client.List(ctx, nodes, &masterNodesMatchingLabels); err != nil {
		err = fmt.Errorf("cannot list nodes: %w", err)
	}

	return
}

func (nc *nodeController) handleNodeNotMaster(ctx context.Context, node *metav1.PartialObjectMetadata) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := nc.deleteNodePKI(ctx, node.Name); err != nil {
		return ctrl.Result{}, err
	}

	/*
		Here we may stop static pods, but it will be a race with k8s scheduler
		So NOOP
	*/

	return ctrl.Result{}, nil
}

func (nc *nodeController) handleNodeDelete(ctx context.Context, name string) (ctrl.Result, error) {
	// Delete node secret if exists
	if err := nc.deleteNodePKI(ctx, name); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (nc *nodeController) deleteNodePKI(ctx context.Context, nodeName string) error {
	log := ctrl.LoggerFrom(ctx)

	secretName := fmt.Sprintf("registry-node-%s-pki", nodeName)
	secret := corev1.Secret{}

	err := nc.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nc.Namespace}, &secret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Already absent
			return nil
		}

		return fmt.Errorf("get node PKI secret error: %w", err)
	}

	err = nc.Client.Delete(ctx, &secret)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete node PKI secret error: %w", err)
	} else {
		log.Info("Deleted node PKI", "node", nodeName, "name", secret.Name, "namespace", secret.Namespace)
	}

	return nil
}

func (nc *nodeController) reprocessChannelSource() source.Source {
	return source.Channel(nc.reprocessCh, handler.TypedEnqueueRequestsFromMapFunc(
		func(_ context.Context, req reconcile.Request) []reconcile.Request {
			return []reconcile.Request{req}
		},
	))
}

func secretObjectIsNodePKI(obj client.Object) bool {
	labels := obj.GetLabels()

	if labels[state.LabelTypeKey] != state.LabelNodeSecretTypeValue {
		return false
	}

	if labels[state.LabelHeritageKey] != state.LabelHeritageValue {
		return false
	}

	return nodePKISecretRegex.MatchString(obj.GetName())
}

func nodeObjectIsMaster(obj client.Object) bool {
	if obj == nil {
		return false
	}

	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	_, hasMasterLabel := labels["node-role.kubernetes.io/master"]

	return hasMasterLabel
}

func hasMasterLabel(node *metav1.PartialObjectMetadata) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	return isMaster
}
