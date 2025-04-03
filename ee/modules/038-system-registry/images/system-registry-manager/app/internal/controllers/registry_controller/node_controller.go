/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/node-services"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"

	"node-services-manager/internal/state"
)

var (
	masterNodesMatchingLabels = client.MatchingLabels{
		state.LabelNodeIsMasterKey: "",
	}
)

type NodeController = nodeController

var _ reconcile.Reconciler = &nodeController{}

type nodeController struct {
	Namespace string
	Client    client.Client

	masterNodeAddrs   []string
	masterNodeAddrsMu sync.Mutex

	eventRecorder record.EventRecorder
	reprocessCh   chan event.TypedGenericEvent[reconcile.Request]
}

var reprocessAllNodesRequest = reconcile.Request{
	NamespacedName: types.NamespacedName{
		Namespace: "--reprocess-all-nodes--",
		Name:      "--reprocess-all-nodes--",
	},
}

func (nc *nodeController) SetupWithManager(mgr ctrl.Manager) error {
	nc.reprocessCh = make(chan event.TypedGenericEvent[reconcile.Request])

	controllerName := "node-controller"
	nc.eventRecorder = mgr.GetEventRecorderFor(controllerName)

	nodePredicate := predicate.Funcs{
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)
			return hasMasterLabel(node)
		},
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)
			return hasMasterLabel(node)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			node := e.Object.(*corev1.Node)
			return hasMasterLabel(node)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldNode := e.ObjectNew.(*corev1.Node)
			newNode := e.ObjectNew.(*corev1.Node)

			if hasMasterLabel(oldNode) != hasMasterLabel(newNode) {
				return true
			}

			if len(oldNode.Status.Addresses) != len(newNode.Status.Addresses) {
				return true
			}

			if getNodeInternalIP(oldNode) != getNodeInternalIP(newNode) {
				return true
			}

			return false
		},
	}

	nodePKISecretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		return state.NodePKISecretRegex.MatchString(obj.GetName())
	})

	nodePKISecretsHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetName()
		sub := state.NodePKISecretRegex.FindStringSubmatch(name)

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
	})

	nodeServicesConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		return strings.HasPrefix(obj.GetName(), state.NodeServicesConfigSecretNamePrefix)
	})

	nodeServicesConfigHander := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetName()

		if !strings.HasPrefix(name, state.NodeServicesConfigSecretNamePrefix) {
			return []reconcile.Request{}
		}

		name = strings.TrimPrefix(name, state.NodeServicesConfigSecretNamePrefix)

		var ret reconcile.Request
		ret.Name = name

		log := ctrl.LoggerFrom(ctx)

		log.Info(
			"NodeServicesConfig secret changed, will trigger reconcile",
			"secret", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"node", ret.Name,
			"controller", controllerName,
		)

		return []reconcile.Request{ret}
	})

	moduleConfig := state.GetModuleConfigObject()
	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == moduleConfig.GetName()
	})

	globalSecretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		name := obj.GetName()
		return name == state.GlobalSecretsName ||
			name == state.PKISecretName ||
			name == state.UserROSecretName ||
			name == state.UserRWSecretName ||
			name == state.UserMirrorPullerName ||
			name == state.UserMirrorPusherName
	})

	stateSecretPredicate := predicate.Funcs{
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			if e.Object.GetNamespace() != nc.Namespace {
				return false
			}

			if e.Object.GetName() != state.StateSecretName {
				return false
			}

			return true
		},
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			if e.Object.GetNamespace() != nc.Namespace {
				return false
			}

			if e.Object.GetName() != state.StateSecretName {
				return false
			}

			return true
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			if e.Object.GetNamespace() != nc.Namespace {
				return false
			}

			if e.Object.GetName() != state.StateSecretName {
				return false
			}

			return true
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			if e.ObjectNew.GetNamespace() != nc.Namespace {
				return false
			}

			if e.ObjectNew.GetName() != state.StateSecretName {
				return false
			}

			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			var oldState, newState state.StateSecret

			if err := oldState.DecodeSecret(oldSecret); err != nil {
				return true
			}

			if err := newState.DecodeSecret(newSecret); err != nil {
				return true
			}

			return oldState != newState
		},
	}

	ingressConfigMapPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		name := obj.GetName()
		return name == state.IngressPKIConfigMapName
	})

	newReprocessAllHandler := func(objectType string) handler.EventHandler {
		return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
			log := ctrl.LoggerFrom(ctx)

			log.Info(
				"Reprocess all nodes will be triggered by object change",
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
				"type", objectType,
				"controller", controllerName,
			)

			return []reconcile.Request{reprocessAllNodesRequest}
		})
	}

	err := ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(
			&corev1.Node{},
			builder.WithPredicates(nodePredicate),
		).
		WatchesRawSource(nc.reprocessChannelSource()).
		Watches(
			&moduleConfig,
			newReprocessAllHandler("ModuleConfig"),
			builder.WithPredicates(moduleConfigPredicate),
		).
		Watches(
			&corev1.Secret{},
			nodePKISecretsHandler,
			builder.WithPredicates(nodePKISecretsPredicate),
		).
		Watches(
			&corev1.Secret{},
			newReprocessAllHandler("Secret"),
			builder.WithPredicates(globalSecretsPredicate),
		).
		Watches(
			&corev1.Secret{},
			newReprocessAllHandler("Secret"),
			builder.WithPredicates(stateSecretPredicate),
		).
		Watches(
			&corev1.Secret{},
			nodeServicesConfigHander,
			builder.WithPredicates(nodeServicesConfigPredicate),
		).
		Watches(
			&corev1.ConfigMap{},
			newReprocessAllHandler("ConfigMap"),
			builder.WithPredicates(ingressConfigMapPredicate),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(nc)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (nc *nodeController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req == reprocessAllNodesRequest {
		return nc.handleReprocessAll(ctx)
	}

	if req.Namespace != "" {
		req.Namespace = ""
	}

	// Delete node secret if exists
	if err := nc.deleteNodePKI(ctx, req.Name); err != nil {
		return ctrl.Result{}, err
	}

	err := nc.checkNodesAddressesChanged(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot check nodes addresses change: %w", err)
	}

	node := &corev1.Node{}
	err = nc.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nc.cleanupNodeState(ctx, node)
		}

		return ctrl.Result{}, fmt.Errorf("cannot get node: %w", err)
	}

	if !hasMasterLabel(node) {
		return nc.cleanupNodeState(ctx, node)
	}

	moduleConfig, err := state.LoadModuleConfig(ctx, nc.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load module config: %w", err)
	}

	if !moduleConfig.Enabled {
		return nc.cleanupNodeState(ctx, node)
	}

	log := ctrl.LoggerFrom(ctx)
	if moduleConfig.Settings.Mode == state.RegistryModeDirect {
		log.Info("Cleanup node for mode = direct")
		return nc.cleanupNodeState(ctx, node)
	}

	return nc.handleMasterNode(ctx, node, moduleConfig)
}

func (nc *nodeController) checkNodesAddressesChanged(ctx context.Context) error {
	ips, err := nc.getAllMasterNodesInternalIPs(ctx)
	if err != nil {
		return fmt.Errorf("cannot get master nodes internal IPs: %w", err)
	}

	log := ctrl.LoggerFrom(ctx)

	nc.masterNodeAddrsMu.Lock()
	defer nc.masterNodeAddrsMu.Unlock()

	if len(ips) != len(nc.masterNodeAddrs) {
		log.Info("Reprocess all nodes will be triggered by master nodes IPs change")

		nc.masterNodeAddrs = ips
		if err = nc.triggerReconcile(ctx, reprocessAllNodesRequest); err != nil {
			return fmt.Errorf("cannot trigger reprocess all nodes: %w", err)
		}
		return nil
	}

	// Addresses already sorted in getAllMasterNodesIPs
	for i := range ips {
		if ips[i] != nc.masterNodeAddrs[i] {
			log.Info("Reprocess all nodes will be triggered by master nodes IPs change")

			nc.masterNodeAddrs = ips
			if err = nc.triggerReconcile(ctx, reprocessAllNodesRequest); err != nil {
				return fmt.Errorf("cannot trigger reprocess all nodes: %w", err)
			}
			return nil
		}
	}

	return nil
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

		if err := nc.triggerReconcile(ctx, req); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (nc *nodeController) triggerReconcile(ctx context.Context, req reconcile.Request) error {
	evt := event.TypedGenericEvent[reconcile.Request]{Object: req}

	select {
	case nc.reprocessCh <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (nc *nodeController) handleMasterNode(ctx context.Context, node *corev1.Node, moduleConfig state.ModuleConfig) (ctrl.Result, error) {
	var (
		result ctrl.Result
		err    error
	)

	log := ctrl.LoggerFrom(ctx).
		WithValues("node", node.Name).
		WithValues("mode", moduleConfig.Settings.Mode)

	userRO, err := nc.loadUserSecret(ctx, state.UserROSecretName)
	if err != nil {
		err = fmt.Errorf("cannot load RO user: %w", err)
		return result, err
	}

	userRW, err := nc.loadUserSecret(ctx, state.UserRWSecretName)
	if err != nil {
		err = fmt.Errorf("cannot load RW user: %w", err)
		return result, err
	}

	userMirrorPuller, err := nc.loadUserSecret(ctx, state.UserMirrorPullerName)
	if err != nil {
		err = fmt.Errorf("cannot load mirror puller user: %w", err)
		return result, err
	}

	userMirrorPusher, err := nc.loadUserSecret(ctx, state.UserMirrorPusherName)
	if err != nil {
		err = fmt.Errorf("cannot load mirror pusher user: %w", err)
		return result, err
	}

	globalSecrets, err := nc.loadGlobalSecrets(ctx)
	if err != nil {
		err = fmt.Errorf("cannot load global secrets: %w", err)
		return result, err
	}

	stateSecret, err := nc.loadStateSecret(ctx)
	if err != nil {
		log.Error(err, "cannot load state secret, will use defaults")
		stateSecret.InitWithDefaults()
	}

	globalPKI, err := nc.loadGlobalPKI(ctx)
	if err != nil {
		err = fmt.Errorf("cannot load global PKI: %w", err)
		return result, err
	}

	ingressPKI, err := nc.loadIngressPKI(ctx)
	if err != nil {
		err = fmt.Errorf("cannot load ingress PKI: %w", err)
		return result, err
	}

	servicesConfig := new(state.NodeServicesConfig)
	*servicesConfig, err = nc.getServicesConfig(ctx, node.Name)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "cannot load Node Service Config secret")
	}

	nodeInternalIP := getNodeInternalIP(node)
	if nodeInternalIP == "" {
		err = fmt.Errorf("node does not have internal IP")
		return result, err
	}

	nodePKI, err := nc.getNodePKI(
		ctx,
		node.Name,
		nodeInternalIP,
		globalPKI,
		servicesConfig,
	)
	if err != nil {
		err = fmt.Errorf("cannot get node PKI: %w", err)
		return result, err
	}

	masterNodesIPs, err := nc.getAllMasterNodesInternalIPs(ctx)
	if err != nil {
		err = fmt.Errorf("cannot get master nodes IPs: %w", err)
		return result, err
	}

	mirrorerUpstreams := make([]string, 0, len(masterNodesIPs))
	for _, ip := range masterNodesIPs {
		if ip != nodeInternalIP {
			mirrorerUpstreams = append(mirrorerUpstreams, ip)
		}
	}

	*servicesConfig, err = nc.contructNodeServicesConfig(
		moduleConfig,
		userRO,
		userRW,
		userMirrorPuller,
		userMirrorPusher,
		globalPKI,
		globalSecrets,
		nodePKI,
		ingressPKI,
		mirrorerUpstreams,
		stateSecret,
	)

	if err != nil {
		err = fmt.Errorf("cannot construct node services configuration: %w", err)
		return result, err
	}

	err = nc.configureNodeServices(ctx, node.Name, *servicesConfig)
	if err != nil {
		err = fmt.Errorf("save node services configuration error: %w", err)
		return result, err
	}

	return result, err
}

func (nc *nodeController) contructNodeServicesConfig(
	moduleConfig state.ModuleConfig,
	userRO, userRW, userMirrorPuller, userMirrorPusher state.User,
	globalPKI state.GlobalPKI,
	globalSecrets state.GlobalSecrets,
	nodePKI state.NodePKI,
	ingressPKI *state.IngressPKI,
	mirrorerUpstreams []string,
	stateSecret state.StateSecret,
) (state.NodeServicesConfig, error) {
	var (
		model state.NodeServicesConfig
		err   error
	)

	tokenKey, err := pki.EncodePrivateKey(globalPKI.Token.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode Token key: %w", err)
		return model, err
	}

	authKey, err := pki.EncodePrivateKey(nodePKI.Auth.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode node's Auth key: %w", err)
		return model, err
	}

	distributionKey, err := pki.EncodePrivateKey(nodePKI.Distribution.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode node's Distribution key: %w", err)
		return model, err
	}

	model = state.NodeServicesConfig{
		Version: stateSecret.Version,
		Config: nodeservices.Config{

			Registry: nodeservices.RegistryConfig{
				HTTPSecret: globalSecrets.HttpSecret,
				UserRO: nodeservices.User{
					Name:         userRO.UserName,
					Password:     userRO.Password,
					PasswordHash: userRO.HashedPassword,
				},
				UserRW: nodeservices.User{
					Name:         userRW.UserName,
					Password:     userRW.Password,
					PasswordHash: userRW.HashedPassword,
				},
			},
			PKI: nodeservices.PKIModel{
				CACert:           string(pki.EncodeCertificate(globalPKI.CA.Cert)),
				TokenCert:        string(pki.EncodeCertificate(globalPKI.Token.Cert)),
				TokenKey:         string(tokenKey),
				AuthCert:         string(pki.EncodeCertificate(nodePKI.Auth.Cert)),
				AuthKey:          string(authKey),
				DistributionCert: string(pki.EncodeCertificate(nodePKI.Distribution.Cert)),
				DistributionKey:  string(distributionKey),
			},
		},
	}

	switch moduleConfig.Settings.Mode {
	case state.RegistryModeProxy:
		host, path := getRegistryAddressAndPathFromImagesRepo(moduleConfig.Settings.Proxy.ImagesRepo)

		model.Config.PKI.UpstreamRegistryCACert = moduleConfig.Settings.Proxy.CA

		model.Config.Registry.Upstream = &nodeservices.UpstreamRegistry{
			Scheme:   strings.ToLower(moduleConfig.Settings.Proxy.Scheme),
			Host:     host,
			Path:     path,
			User:     moduleConfig.Settings.Proxy.UserName,
			Password: moduleConfig.Settings.Proxy.Password,
			TTL:      moduleConfig.Settings.Proxy.TTL.StringPointer(),
		}
	case state.RegistryModeDetached:
		model.Config.Registry.Mirrorer = &nodeservices.Mirrorer{
			UserPuller: nodeservices.User{
				Name:         userMirrorPuller.UserName,
				Password:     userMirrorPuller.Password,
				PasswordHash: userMirrorPuller.HashedPassword,
			},
			UserPusher: nodeservices.User{
				Name:         userMirrorPusher.UserName,
				Password:     userMirrorPusher.Password,
				PasswordHash: userMirrorPusher.HashedPassword,
			},
			Upstreams: mirrorerUpstreams,
		}
	}

	if ingressPKI != nil {
		model.Config.PKI.IngressClientCACert = string(pki.EncodeCertificate(ingressPKI.ClientCACert))
	}

	return model, err
}

func (nc *nodeController) configureNodeServices(ctx context.Context, nodeName string, config state.NodeServicesConfig) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "ConfigureNodeServices")

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.NodeServicesConfigSecretName(nodeName),
		Namespace: nc.Namespace,
	}

	var origSecret *corev1.Secret

	if err := nc.Client.Get(ctx, key, &secret); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		}
	} else {
		// Making a copy unconditionally is a bit wasteful, since we don't
		// always need to update the service. But, making an unconditional
		// copy makes the code much easier to follow, and we have a GC for
		// a reason.
		origSecret = secret.DeepCopy()
	}

	if err := config.EncodeSecret(&secret); err != nil {
		return fmt.Errorf("cannot encode to secret: %w", err)
	}

	if origSecret == nil {
		secret.Name = key.Name
		secret.Namespace = key.Namespace

		if err := nc.Client.Create(ctx, &secret); err != nil {
			return fmt.Errorf("cannot save secret %v for NodeServicesConfig: %w", secret.Name, err)
		}
	} else {
		// Type cannot be changed, so preserve original value
		secret.Type = origSecret.Type

		// Check than we're need to update secret
		if !reflect.DeepEqual(origSecret, secret) {
			if err := nc.Client.Update(ctx, &secret); err != nil {
				return fmt.Errorf("cannot update secret %v for NodeServicesConfig: %w", secret.Name, err)
			}
		} else {
			log.Info("No changes in Node Services Config needed")
		}
	}

	return nil
}

func (nc *nodeController) stopNodeServices(ctx context.Context, nodeName string) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "DeleteNodeServicesConfig")

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.NodeServicesConfigSecretName(nodeName),
		Namespace: nc.Namespace,
	}

	err := nc.Client.Get(ctx, key, &secret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Already absent
			return nil
		}

		return fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	err = nc.Client.Delete(ctx, &secret)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete secret %v error: %w", secret.Name, err)
	}

	log.Info("Deleted Node Services config", "node", nodeName, "name", secret.Name, "namespace", secret.Namespace)
	return nil
}

func (nc *nodeController) getServicesConfig(ctx context.Context, nodeName string) (state.NodeServicesConfig, error) {
	var (
		ret state.NodeServicesConfig
		err error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.NodeServicesConfigSecretName(nodeName),
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return ret, err
	}

	err = ret.DecodeSecret(&secret)
	if err != nil {
		err = fmt.Errorf("cannot decode from secret: %w", err)
		return ret, err
	}

	err = ret.Validate()
	if err != nil {
		err = fmt.Errorf("valdiation error: %w", err)
		return ret, err
	}

	return ret, err
}

func (nc *nodeController) getNodePKI(
	ctx context.Context,
	nodeName,
	nodeAddress string,
	globalPKI state.GlobalPKI,
	servicesConfig *state.NodeServicesConfig,
) (state.NodePKI, error) {
	var (
		ret state.NodePKI
		err error
	)

	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureNodePKI")

	hosts := []string{
		"127.0.0.1",
		"localhost",
		nodeAddress,
		fmt.Sprintf("%s.%s.svc", state.RegistrySvcName, state.RegistryNamespace),
	}

	if servicesConfig != nil {
		if ret, err = nc.loadNodePKIFromConfig(*servicesConfig, globalPKI, hosts); err == nil {
			return ret, err
		}

		nc.logModuleWarning(
			&log,
			"NodePKIDecodeError",
			fmt.Sprintf("Error decode Node PKI: %v", err),
		)
	}

	nc.logModuleWarning(
		&log,
		fmt.Sprintf("NodePKIGenerateNew: %v", nodeName),
		"Generating new NodePKI",
	)

	if ret, err = state.GenerateNodePKI(*globalPKI.CA, hosts); err != nil {
		err = fmt.Errorf("cannot generate new PKI: %w", err)
	}

	return ret, err
}

func (nc *nodeController) loadNodePKIFromConfig(
	servicesConfig state.NodeServicesConfig,
	globalPKI state.GlobalPKI,
	hosts []string,
) (state.NodePKI, error) {
	var (
		ret state.NodePKI
		err error
	)

	err = ret.DecodeServicesConfig(servicesConfig)

	if err != nil {
		err = fmt.Errorf("cannot decode Node PKI from config: %w", err)
		return ret, err
	}

	err = pki.ValidateCertWithCAChain(ret.Auth.Cert, globalPKI.CA.Cert)
	if err != nil {
		err = fmt.Errorf("error validating Auth certificate: %w", err)
		return ret, err
	}

	err = pki.ValidateCertWithCAChain(ret.Distribution.Cert, globalPKI.CA.Cert)
	if err != nil {
		err = fmt.Errorf("error validating Distribution certificate: %w", err)
		return ret, err
	}

	for _, host := range hosts {
		if err = ret.Auth.Cert.VerifyHostname(host); err != nil {
			err = fmt.Errorf("hostname \"%v\" not supported by Auth certificate: %w", host, err)
			return ret, err
		}

		if err = ret.Distribution.Cert.VerifyHostname(host); err != nil {
			err = fmt.Errorf("hostname \"%v\" not supported by Distribution certificate: %w", host, err)
			return ret, err
		}
	}

	return ret, err
}

func (nc *nodeController) loadGlobalSecrets(ctx context.Context) (state.GlobalSecrets, error) {
	var (
		ret state.GlobalSecrets
		err error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.GlobalSecretsName,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return ret, err
	}

	if err = ret.DecodeSecret(&secret); err != nil {
		err = fmt.Errorf("cannot decode from secret: %w", err)
		return ret, err
	}

	if err = ret.Validate(); err != nil {
		err = fmt.Errorf("valdiation error: %w", err)
	}

	return ret, err
}

func (nc *nodeController) loadStateSecret(ctx context.Context) (state.StateSecret, error) {
	var (
		ret state.StateSecret
		err error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.StateSecretName,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return ret, err
	}

	if err = ret.DecodeSecret(&secret); err != nil {
		err = fmt.Errorf("cannot decode from secret: %w", err)
		return ret, err
	}

	if err = ret.Validate(); err != nil {
		err = fmt.Errorf("valdiation error: %w", err)
		return ret, err
	}

	return ret, err
}

func (nc *nodeController) loadUserSecret(ctx context.Context, name string) (state.User, error) {
	var (
		ret state.User
		err error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return ret, err
	}

	if err = ret.DecodeSecret(&secret); err != nil {
		err = fmt.Errorf("cannot decode from secret: %w", err)
		return ret, err
	}

	if !ret.IsValid() {
		err = fmt.Errorf("user data is invalid")
		return ret, err
	}

	if !ret.IsPasswordHashValid() {
		err = fmt.Errorf("password hash not corresponding to password")
		return ret, err
	}

	return ret, err
}

func (nc *nodeController) loadGlobalPKI(ctx context.Context) (state.GlobalPKI, error) {
	var (
		ret state.GlobalPKI
		err error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.PKISecretName,
		Namespace: nc.Namespace,
	}

	if err = nc.Client.Get(ctx, key, &secret); err != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return ret, err
	}

	err = ret.DecodeSecret(&secret)
	if err != nil {
		err = fmt.Errorf("cannot decode PKI from secret: %w", err)
		return ret, err
	}

	err = ret.Validate()
	if err != nil {
		err = fmt.Errorf("cannot validate PKI: %w", err)
		return ret, err
	}

	return ret, err
}

func (nc *nodeController) loadIngressPKI(ctx context.Context) (*state.IngressPKI, error) {
	cm := corev1.ConfigMap{}
	key := types.NamespacedName{
		Name:      state.IngressPKIConfigMapName,
		Namespace: nc.Namespace,
	}

	if err := nc.Client.Get(ctx, key, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot get configmap %v k8s object: %w", key.Name, err)
	}

	ret := state.IngressPKI{}
	err := ret.DecodeConfigMap(&cm)
	if err != nil {
		return nil, fmt.Errorf("cannot decode PKI from configmap %v: %w", key.Name, err)
	}
	return &ret, nil
}

func (nc *nodeController) getAllMasterNodesInternalIPs(ctx context.Context) ([]string, error) {
	var (
		ips []string
		err error
	)

	nodes, err := nc.getAllMasterNodes(ctx)
	if err != nil {
		return ips, err
	}

	for _, node := range nodes.Items {
		if ip := getNodeInternalIP(&node); ip != "" {
			ips = append(ips, ip)
		}
	}

	sort.Strings(ips)
	return ips, err
}

func (nc *nodeController) getAllMasterNodes(ctx context.Context) (*corev1.NodeList, error) {
	var err error
	nodes := &corev1.NodeList{}

	if err = nc.Client.List(ctx, nodes, &masterNodesMatchingLabels); err != nil {
		err = fmt.Errorf("cannot list nodes: %w", err)
	}

	return nodes, err
}

func (nc *nodeController) cleanupNodeState(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	if err := nc.stopNodeServices(ctx, node.Name); err != nil {
		err = fmt.Errorf("delete Node Services Config error: %w", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (nc *nodeController) deleteNodePKI(ctx context.Context, nodeName string) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "DeleteNodePKI")

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.NodePKISecretName(nodeName),
		Namespace: nc.Namespace,
	}

	err := nc.Client.Get(ctx, key, &secret)

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
	}

	log.Info("Deleted node PKI", "node", nodeName, "name", secret.Name, "namespace", secret.Namespace)
	return nil
}

func (nc *nodeController) reprocessChannelSource() source.Source {
	return source.Channel(nc.reprocessCh, handler.TypedEnqueueRequestsFromMapFunc(
		func(_ context.Context, req reconcile.Request) []reconcile.Request {
			return []reconcile.Request{req}
		},
	))
}

func (nc *nodeController) logModuleWarning(log *logr.Logger, reason, message string) {
	obj := state.GetModuleConfigObject()
	obj.SetNamespace(nc.Namespace)

	nc.eventRecorder.Event(&obj, corev1.EventTypeWarning, reason, message)

	if log != nil {
		log.Info(message, "reason", reason)
	}
}

func hasMasterLabel(node *corev1.Node) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	return isMaster
}

func getNodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}
