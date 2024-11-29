/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"embeded-registry-manager/internal/state"
	"embeded-registry-manager/internal/staticpod"
	httpclient "embeded-registry-manager/internal/utils/http_client"
	"embeded-registry-manager/internal/utils/pki"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

const (
	staticPodURLFormat = "https://%s:4577/staticpod"
	registryHttpSecret = "http-secret"
)

var (
	masterNodesMatchingLabels = client.MatchingLabels{
		state.LabelNodeIsMasterKey: "",
	}

	staticPodMatchingLabels = client.MatchingLabels{
		"app": "system-registry-staticpod-manager",
	}
)

type NodeController = nodeController

var _ reconcile.Reconciler = &nodeController{}

type NodeControllerSettings struct {
	RegistryAddress   string
	RegistryPath      string
	ImageAuth         string
	ImageDistribution string
}

type nodeController struct {
	Namespace  string
	Client     client.Client
	HttpClient *httpclient.Client
	Settings   NodeControllerSettings

	eventRecorder record.EventRecorder
	reprocessCh   chan event.TypedGenericEvent[reconcile.Request]
}

var nodeReprocessAllRequest = reconcile.Request{
	NamespacedName: types.NamespacedName{
		Namespace: "--reprocess-all-nodes--",
		Name:      "--reprocess-all-nodes--",
	},
}

func (nc *nodeController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	nc.reprocessCh = make(chan event.TypedGenericEvent[reconcile.Request])

	controllerName := "node-controller"
	nc.eventRecorder = mgr.GetEventRecorderFor(controllerName)

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
			return nodeObjectIsMaster(e.Object)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return nodeObjectIsMaster(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			return nodeObjectIsMaster(e.ObjectOld) != nodeObjectIsMaster(e.ObjectNew)
		},
	}

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		return state.NodePKISecretRegex.MatchString(obj.GetName())
	})

	secretsHandler := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		name := obj.GetName()
		sub := state.NodePKISecretRegex.FindStringSubmatch(name)

		if sub == nil || len(sub) < 2 {
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

	moduleConfig := state.GetModuleConfigObject()
	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == state.RegistryModuleName
	})

	globalSecretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		name := obj.GetName()
		return name == state.PKISecretName || name == state.UserROSecretName || name == state.UserRWSecretName
	})

	staticPodManagerPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != nc.Namespace {
			return false
		}

		labels := obj.GetLabels()
		for k, v := range staticPodMatchingLabels {
			if labels[k] != v {
				return false
			}
		}

		return true
	})

	staticPodManagerHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		pod, ok := obj.(*corev1.Pod)

		if !ok {
			return nil
		}

		var ret reconcile.Request
		ret.Name = pod.Spec.NodeName
		return []reconcile.Request{ret}
	})

	reprocessAllHandler := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{nodeReprocessAllRequest}
	})

	err = ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(
			&corev1.Node{},
			builder.WithPredicates(nodePredicate),
		).
		Watches(
			&corev1.Secret{},
			secretsHandler,
			builder.WithPredicates(secretsPredicate),
		).
		Watches(
			&corev1.Pod{},
			staticPodManagerHandler,
			builder.WithPredicates(staticPodManagerPredicate),
		).
		WatchesRawSource(nc.reprocessChannelSource()).
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
		Complete(nc)

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

	node := &corev1.Node{}

	err := nc.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nc.handleNodeDelete(ctx, req.Name)
		}

		return ctrl.Result{}, fmt.Errorf("cannot get node: %w", err)
	}

	moduleConfig, err := state.LoadModuleConfig(ctx, nc.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load module config: %w", err)
	}

	if !moduleConfig.Enabled {
		return ctrl.Result{}, nil
	}

	log := ctrl.LoggerFrom(ctx)
	if moduleConfig.Settings.Mode == state.RegistryModeDirect {
		log.Info("Cleanup node for mode = direct")
		return nc.cleanupNodeState(ctx, node)
	}

	if hasMasterLabel(node) {
		return nc.handleMasterNode(ctx, node, moduleConfig)
	} else {
		return nc.cleanupNodeState(ctx, node)
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

func (nc *nodeController) handleMasterNode(ctx context.Context, node *corev1.Node, moduleConfig state.ModuleConfig) (result ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("node", node.Name).
		WithValues("mode", moduleConfig.Settings.Mode)

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

	globalPKI, err := nc.loadGlobalPKI(ctx)
	if err != nil {
		err = fmt.Errorf("cannot load global PKI: %w", err)
		return
	}

	if len(node.Status.Addresses) == 0 {
		err = fmt.Errorf("node does not have address")
		return
	}

	nodePKI, err := nc.ensureNodePKI(
		ctx,
		node.Name,
		node.Status.Addresses[0].Address,
		globalPKI,
	)
	if err != nil {
		err = fmt.Errorf("cannot ensure node PKI: %w", err)
		return
	}

	staticPodConfig, err := nc.contructStaticPodConfig(
		moduleConfig,
		userRO,
		userRW,
		globalPKI,
		nodePKI,
	)

	if err != nil {
		err = fmt.Errorf("cannot construct static pod config: %w", err)
		return
	}

	if moduleConfig.Settings.Mode == state.RegistryModeDetached {
		var isFirstMasterNode bool

		isFirstMasterNode, err = nc.isFirstMasterNode(ctx, node)
		if err != nil {
			err = fmt.Errorf("cannot check node is first master node: %w", err)
			return
		}
		log = log.WithValues("firstMasterNode", isFirstMasterNode)

		if isFirstMasterNode {
			log.Info("Processing first master node for mode == detached")
			err = nc.applyStaticPodConfig(ctx, node.Name, staticPodConfig)
			if err != nil {
				err = fmt.Errorf("apply static pod configuration error: %w", err)
			}

			return
		}

		log.Info("Shutdown node static pod on non-master node for mode = detached")
		err = nc.deleteStaticPodConfig(ctx, node.Name)
		if err != nil {
			err = fmt.Errorf("delete static pod configuration error: %w", err)
			return
		}

		return
	}

	log.Info("Processing master node")
	err = nc.applyStaticPodConfig(ctx, node.Name, staticPodConfig)
	if err != nil {
		err = fmt.Errorf("apply static pod configuration error: %w", err)
		return
	}

	return
}

func (nc *nodeController) contructStaticPodConfig(moduleConfig state.ModuleConfig, userRO, userRW state.User, globalPKI state.GlobalPKI, nodePKI state.NodePKI) (config staticpod.Config, err error) {
	tokenKey, err := pki.EncodePrivateKey(globalPKI.Token.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode Token key: %w", err)
		return
	}

	authKey, err := pki.EncodePrivateKey(nodePKI.Auth.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode node's Auth key: %w", err)
		return
	}

	distributionKey, err := pki.EncodePrivateKey(nodePKI.Distribution.Key)
	if err != nil {
		err = fmt.Errorf("cannot encode node's Distribution key: %w", err)
		return
	}

	config = staticpod.Config{
		Images: staticpod.Images{
			Auth:         nc.Settings.ImageAuth,
			Distribution: nc.Settings.ImageDistribution,
		},
		Registry: staticpod.RegistryConfig{
			Mode:       staticpod.RegistryMode(moduleConfig.Settings.Mode),
			HttpSecret: registryHttpSecret,
			UserRO: staticpod.User{
				Name:         userRO.UserName,
				PasswordHash: userRO.HashedPassword,
			},
			UserRW: staticpod.User{
				Name:         userRW.UserName,
				PasswordHash: userRW.HashedPassword,
			},
		},
		PKI: staticpod.PKIModel{
			CACert:           string(pki.EncodeCertificate(globalPKI.CA.Cert)),
			TokenCert:        string(pki.EncodeCertificate(globalPKI.Token.Cert)),
			TokenKey:         string(tokenKey),
			AuthCert:         string(pki.EncodeCertificate(nodePKI.Auth.Cert)),
			AuthKey:          string(authKey),
			DistributionCert: string(pki.EncodeCertificate(nodePKI.Distribution.Cert)),
			DistributionKey:  string(distributionKey),
		},
	}

	if moduleConfig.Settings.Mode == state.RegistryModeProxy {
		config.Registry.Upstream = staticpod.UpstreamRegistry{
			Scheme:   moduleConfig.Settings.Proxy.Scheme,
			Host:     moduleConfig.Settings.Proxy.Host,
			Path:     moduleConfig.Settings.Proxy.Path,
			CA:       moduleConfig.Settings.Proxy.CA,
			User:     moduleConfig.Settings.Proxy.User,
			Password: moduleConfig.Settings.Proxy.Password,
			TTL:      moduleConfig.Settings.Proxy.TTL.StringPointer(),
		}
	}

	return
}

func (nc *nodeController) applyStaticPodConfig(ctx context.Context, nodeName string, config staticpod.Config) error {
	podIP, err := nc.findStaticPodManagerIP(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("cannot find Static Pod Manager IP for Node: %w", err)
	}

	url := fmt.Sprintf(staticPodURLFormat, podIP)
	_, err = nc.HttpClient.SendJSON(url, http.MethodPost, config)

	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}

	return nil
}

func (nc *nodeController) deleteStaticPodConfig(ctx context.Context, nodeName string) error {
	// TODO: return ok if not found?
	podIP, err := nc.findStaticPodManagerIP(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("cannot find Static Pod Manager IP for Node: %w", err)
	}

	url := fmt.Sprintf(staticPodURLFormat, podIP)
	_, err = nc.HttpClient.SendJSON(url, http.MethodDelete, nil)

	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}

	return nil
}

func (nc *nodeController) findStaticPodManagerIP(ctx context.Context, nodeName string) (string, error) {
	var pods corev1.PodList

	err := nc.Client.List(
		ctx,
		&pods,
		staticPodMatchingLabels,
		client.MatchingFields{
			"spec.nodeName": nodeName,
		},
	)

	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("system-registry-staticpod-manager pod not found for node %s", nodeName)
	}
	if pods.Items[0].Status.PodIP == "" {
		return "", fmt.Errorf("system-registry-staticpod-manager pod IP is empty for node %s", nodeName)
	}

	return pods.Items[0].Status.PodIP, nil
}

func (nc *nodeController) ensureNodePKI(ctx context.Context, nodeName, nodeAddress string, globalPKI state.GlobalPKI) (ret state.NodePKI, err error) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureNodePKI")

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.NodePKISecretName(nodeName),
		Namespace: nc.Namespace,
	}

	err = nc.Client.Get(ctx, key, &secret)
	if client.IgnoreNotFound(err) != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	secretOrig := secret.DeepCopy()

	hosts := []string{
		"127.0.0.1",
		"localhost",
		nodeAddress,
		fmt.Sprintf("%s.%s.svc", state.RegistrySvcName, state.RegistryNamespace),
	}

	notFound := false
	isValid := true
	if err != nil {
		notFound = true
	} else {
		ret, isValid = nc.loadNodePKIFromSecret(log, &secret, &globalPKI, hosts)
	}

	if notFound || !isValid {
		if notFound {
			nc.logModuleWarning(
				&log,
				"NodePKINotfound",
				"NodePKI secret not found, will generate new",
			)
		} else {
			nc.logModuleWarning(
				&log,
				"NodePKIInvalid",
				"NodePKI secret invalid, will generate new",
			)
		}

		var generatedPKI pki.CertKey

		generatedPKI, err = pki.GenerateCertificate(state.NodeAuthCertCN, hosts, *globalPKI.CA)
		if err != nil {
			err = fmt.Errorf("cannot generate Auth PKI: %w", err)
			return
		}
		ret.Auth = &generatedPKI

		generatedPKI, err = pki.GenerateCertificate(state.NodeDistributionCertCN, hosts, *globalPKI.CA)
		if err != nil {
			err = fmt.Errorf("cannot generate Distribution PKI: %w", err)
			return
		}
		ret.Distribution = &generatedPKI
	}

	// Set labels
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}

	secret.Labels[state.LabelModuleKey] = state.RegistryModuleName
	secret.Labels[state.LabelHeritageKey] = state.LabelHeritageValue
	secret.Labels[state.LabelManagedBy] = state.RegistryModuleName
	secret.Labels[state.LabelTypeKey] = state.NodePKISecretTypeLabel

	secret.Data = make(map[string][]byte)
	if err = state.EncodeCertKeyToSecret(
		*ret.Auth,
		state.NodeAuthCertSecretField,
		state.NodeAuthKeySecretField,
		&secret,
	); err != nil {
		err = fmt.Errorf("cannot encode Auth NodePKI to secret: %w", err)
		return
	}

	if err = state.EncodeCertKeyToSecret(
		*ret.Distribution,
		state.NodeDistributionCertSecretField,
		state.NodeDistributionKeySecretField,
		&secret,
	); err != nil {
		err = fmt.Errorf("cannot encode Distribution NodePKI to secret: %w", err)
		return
	}

	if notFound {
		secret.Name = key.Name
		secret.Namespace = key.Namespace
		secret.Type = state.NodePKISecretType

		if err = nc.Client.Create(ctx, &secret); err != nil {
			err = fmt.Errorf("cannot create k8s object: %w", err)
			return
		}

		log.Info("New secret was created")
	} else {
		// Check than we're need to update secret
		if !reflect.DeepEqual(secretOrig, secret) {
			if err = nc.Client.Update(ctx, &secret); err != nil {
				err = fmt.Errorf("cannot update k8s object: %w", err)
				return
			}

			if secretOrig.ResourceVersion != secret.ResourceVersion {
				log.Info("Secret was updated")
			}

		}
	}

	return
}

func (nc *nodeController) loadNodePKIFromSecret(log logr.Logger, secret *corev1.Secret, globalPKI *state.GlobalPKI, hosts []string) (ret state.NodePKI, isValid bool) {
	authPKI, err := state.DecodeCertKeyFromSecret(
		state.NodeAuthCertSecretField,
		state.NodeAuthKeySecretField,
		secret,
	)

	if err != nil {
		log.Error(err, "Cannot decode auth PKI")

		nc.logModuleWarning(
			&log,
			"NodePKIAuthDecodeError",
			fmt.Sprintf("NodePKI Auth decode error: %v", err),
		)

		return
	}
	ret.Auth = &authPKI

	err = pki.ValidateCertWithCAChain(ret.Auth.Cert, globalPKI.CA.Cert)
	if err != nil {
		log.Error(err, "Auth certificate validation error")

		nc.logModuleWarning(
			&log,
			"NodePKIAuthCertValidationError",
			fmt.Sprintf("NodePKI Auth certificate validation error: %v", err),
		)

		return
	}

	for _, host := range hosts {
		err = ret.Auth.Cert.VerifyHostname(host)
		if err != nil {
			log.Error(err, "Hostname not supported by Auth certificate", "hostName", host)

			nc.logModuleWarning(
				&log,
				"NodePKIAuthCertHostUnsupported",
				fmt.Sprintf("NodePKI Auth certificate not support hostname %v: %v", host, err),
			)

			return
		}
	}

	distributionPKI, err := state.DecodeCertKeyFromSecret(
		state.NodeDistributionCertSecretField,
		state.NodeDistributionKeySecretField,
		secret,
	)

	if err != nil {
		log.Error(err, "Cannot decode Distribution PKI")

		nc.logModuleWarning(
			&log,
			"NodePKIDistributionDecodeError",
			fmt.Sprintf("NodePKI Distribution decode error: %v", err),
		)

		return
	}
	ret.Distribution = &distributionPKI

	err = pki.ValidateCertWithCAChain(ret.Distribution.Cert, globalPKI.CA.Cert)
	if err != nil {
		log.Error(err, "Distribution certificate validation error")

		nc.logModuleWarning(
			&log,
			"NodePKIDistributionCertValidationError",
			fmt.Sprintf("NodePKI Distribution certificate validation error: %v", err),
		)

		return
	}

	for _, host := range hosts {
		err = ret.Distribution.Cert.VerifyHostname(host)
		if err != nil {
			log.Error(err, "Hostname not supported by distribution certificate", "hostName", host)

			nc.logModuleWarning(
				&log,
				"NodePKIDistributionCertHostUnsupported",
				fmt.Sprintf("NodePKI Distribution certificate not support hostname %v: %v", host, err),
			)

			return
		}
	}

	isValid = true
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

func (nc *nodeController) loadGlobalPKI(ctx context.Context) (ret state.GlobalPKI, err error) {
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

func (nc *nodeController) isFirstMasterNode(ctx context.Context, node *corev1.Node) (bool, error) {
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

func (nc *nodeController) cleanupNodeState(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	// Delete static pod (let's race with k8s sheduler)
	if err := nc.deleteStaticPodConfig(ctx, node.Name); err != nil {
		err = fmt.Errorf("delete static pod configuration error: %w", err)
		return ctrl.Result{}, err
	}

	// Delete node secret if exists
	if err := nc.deleteNodePKI(ctx, node.Name); err != nil {
		return ctrl.Result{}, err
	}

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

func (nc *nodeController) logModuleWarning(log *logr.Logger, reason, message string) {
	obj := state.GetModuleConfigObject()
	obj.SetNamespace(nc.Namespace)

	nc.eventRecorder.Event(&obj, corev1.EventTypeWarning, reason, message)

	if log != nil {
		log.Info(message, "reason", reason)
	}
}

func (nc *nodeController) logModuleInfo(log *logr.Logger, reason, message string) {
	obj := state.GetModuleConfigObject()
	obj.SetNamespace(nc.Namespace)

	nc.eventRecorder.Event(&obj, corev1.EventTypeNormal, reason, message)

	if log != nil {
		log.Info(message, "reason", reason)
	}
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

func hasMasterLabel(node *corev1.Node) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	return isMaster
}
