/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controllers

import (
	"context"
	staticpod "embeded-registry-manager/internal/static-pod"
	httpclient "embeded-registry-manager/internal/utils/http_client"
	"embeded-registry-manager/internal/utils/k8s"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"net/http"
	"os"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"strings"
	"sync"
)

type ModuleConfig struct {
	Enabled  bool           `json:"enabled"`
	Settings RegistryConfig `json:"settings"`
}

type RegistryConfig struct {
	Mode     string          `json:"mode"` // enum: Direct, Proxy, Detached
	Proxy    *ProxyConfig    `json:"proxy,omitempty"`
	Detached *DetachedConfig `json:"detached,omitempty"`
}

type StorageMode string

type DetachedConfig struct {
	StorageMode StorageMode `json:"storageMode"` // enum: S3, Fs
}

type ProxyConfig struct {
	Host        string      `json:"host"`
	Scheme      string      `json:"scheme"`
	CA          string      `json:"ca"`
	Path        string      `json:"path"`
	User        string      `json:"user"`
	Password    string      `json:"password"`
	StorageMode StorageMode `json:"storageMode"` // enum: S3, Fs
}

type embeddedRegistry struct {
	Mutex          sync.Mutex
	mc             ModuleConfig
	caPKI          k8s.Certificate
	authTokenPKI   k8s.Certificate
	registryRwUser k8s.RegistryUser
	registryRoUser k8s.RegistryUser
	masterNodes    []k8s.MasterNode
	images         staticpod.Images
}

type RegistryReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	KubeClient       *kubernetes.Clientset
	Recorder         record.EventRecorder
	HttpClient       *httpclient.Client
	embeddedRegistry embeddedRegistry
}

var nodePKISecretRegex = regexp.MustCompile(`^registry-node-.*-pki$`)

// SetupWithManager sets up the controller with the Manager to watch for changes in both Pods and Secrets.
func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {

	// Set up the field indexer to index Pods by spec.nodeName
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
		pod := obj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return fmt.Errorf("failed to set up index on spec.nodeName: %w", err)
	}

	// Set up moduleConfig informer
	moduleConfig := &unstructured.Unstructured{}
	moduleConfig.SetAPIVersion(k8s.ModuleConfigApiVersion)
	moduleConfig.SetKind(k8s.ModuleConfigKind)

	moduleConfigInformer, err := mgr.GetCache().GetInformer(ctx, moduleConfig)
	if err != nil {
		return fmt.Errorf("unable to get informer for ModuleConfig: %w", err)
	}

	// Add event handler for ModuleConfig
	_, err = moduleConfigInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				unstructuredObj, ok := obj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigCreate(ctx, obj)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				unstructuredObj, ok := newObj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigChange(ctx, newObj)
				}

			},
			DeleteFunc: func(obj interface{}) {
				unstructuredObj, ok := obj.(*unstructured.Unstructured)
				if ok && unstructuredObj.GetName() == k8s.RegistryMcName {
					r.handleModuleConfigDelete(ctx)
				}
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to add event handler for ModuleConfig: %w", err)
	}

	bldr := ctrl.NewControllerManagedBy(mgr)
	bldr.Named("embedded-registry-controller")

	// Watch for changes in Secrets
	err = bldr.Watches(&corev1.Secret{}, r.secretEventHandler()).WithOptions(controller.Options{
		MaxConcurrentReconciles: 1,
	}).Complete(r)
	if err != nil {
		return fmt.Errorf("unable to complete controller: %w", err)
	}

	return nil
}

func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//logger := ctrl.LoggerFrom(ctx)
	// Lock the mutex for the embedded registry struct to prevent simultaneous writes
	r.embeddedRegistry.Mutex.Lock()
	defer r.embeddedRegistry.Mutex.Unlock()

	switch {
	// Check if the secret is the registry-pki secret
	case req.NamespacedName.Name == "registry-pki":
		err := r.handleRegistryCaPKI(ctx, req)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	// Check if the secret is the registry-node-*-pki secret
	case nodePKISecretRegex.MatchString(req.NamespacedName.Name):
		nodeName := strings.TrimPrefix(strings.TrimSuffix(req.NamespacedName.Name, "-pki"), "registry-node-")

		node, err := r.handleNodePKI(ctx, req, nodeName)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.syncRegistryStaticPods(ctx, node)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	// Check if the secret is the registry-user-rw secret
	case req.NamespacedName.Name == "registry-user-rw":
		_, err := r.handleRegistryUser(ctx, req, "registry-user-rw", &r.embeddedRegistry.registryRwUser)
		if err != nil {
			return ctrl.Result{}, err
		}
	// Check if the secret is the registry-user-ro secret
	case req.NamespacedName.Name == "registry-user-ro":
		_, err := r.handleRegistryUser(ctx, req, "registry-user-ro", &r.embeddedRegistry.registryRoUser)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	for _, node := range r.embeddedRegistry.masterNodes {
		_ = r.syncRegistryStaticPods(ctx, node)
	}

	return ctrl.Result{}, nil
}

func (r *RegistryReconciler) syncRegistryStaticPods(ctx context.Context, node k8s.MasterNode) error {
	logger := ctrl.LoggerFrom(ctx)

	upstreamRegistry := staticpod.UpstreamRegistry{}

	if r.embeddedRegistry.mc.Settings.Mode == "Proxy" {
		upstreamRegistry.Scheme = r.embeddedRegistry.mc.Settings.Proxy.Scheme
		upstreamRegistry.Host = r.embeddedRegistry.mc.Settings.Proxy.Host
		upstreamRegistry.Path = r.embeddedRegistry.mc.Settings.Proxy.Path
		upstreamRegistry.CA = r.embeddedRegistry.mc.Settings.Proxy.CA
		upstreamRegistry.User = r.embeddedRegistry.mc.Settings.Proxy.User
		upstreamRegistry.Password = r.embeddedRegistry.mc.Settings.Proxy.Password
	}

	data := staticpod.EmbeddedRegistryConfig{
		Registry: staticpod.RegistryDetails{
			UserRw: staticpod.User{
				Name:         r.embeddedRegistry.registryRwUser.UserName,
				PasswordHash: r.embeddedRegistry.registryRwUser.HashedPassword,
			},
			UserRo: staticpod.User{
				Name:         r.embeddedRegistry.registryRoUser.UserName,
				PasswordHash: r.embeddedRegistry.registryRoUser.HashedPassword,
			},
			RegistryMode:     r.embeddedRegistry.mc.Settings.Mode,
			HttpSecret:       "http-secret",
			UpstreamRegistry: upstreamRegistry,
		},
		Images: staticpod.Images{
			DockerDistribution: r.embeddedRegistry.images.DockerDistribution,
			DockerAuth:         r.embeddedRegistry.images.DockerAuth,
		},
		Pki: staticpod.Pki{
			CaCert:           string(r.embeddedRegistry.caPKI.Cert),
			AuthCert:         string(node.AuthCertificate.Cert),
			AuthKey:          string(node.AuthCertificate.Key),
			AuthTokenCert:    string(r.embeddedRegistry.authTokenPKI.Cert),
			AuthTokenKey:     string(r.embeddedRegistry.authTokenPKI.Key),
			DistributionCert: string(node.DistributionCertificate.Cert),
			DistributionKey:  string(node.DistributionCertificate.Key),
		},
	}

	var pods corev1.PodList

	err := r.List(ctx, &pods, client.MatchingLabels{
		"app": "system-registry-manager",
	}, client.MatchingFields{
		"spec.nodeName": node.Name,
	})

	if err != nil {
		logger.Error(err, "Failed to list pods", "node", node.Name)
		return err
	}
	if len(pods.Items) == 0 {
		logger.Error(fmt.Errorf("system-registry-manager pod not found"), "system-registry-manager pod not found", "node", node.Name)
		return err
	}
	if pods.Items[0].Status.PodIP == "" {
		logger.Error(fmt.Errorf("system-registry-manager pod IP is empty"), "system-registry-manager pod IP is empty", "node", node.Name)
		return err
	}

	response, err := r.HttpClient.Send(fmt.Sprintf("https://%s:4577/staticpod/create", pods.Items[0].Status.PodIP), http.MethodPost, data)
	if err != nil {
		logger.Info("Failed to reconcile registry", "node", node.Name, "error", err)
	} else {
		logger.Info("Reconcile registry", "node", node.Name, "response", string(response))
	}

	return nil
}

func (r *embeddedRegistry) getMasterNodeFromEmbeddedRegistryStruct(nodeName string) (k8s.MasterNode, bool) {
	for i, node := range r.masterNodes {
		if node.Name == nodeName {
			return r.masterNodes[i], true
		}
	}
	return k8s.MasterNode{}, false
}

func (r *embeddedRegistry) updateMasterNode(masterNode k8s.MasterNode) {
	// Update the node in the embedded registry struct if it exists
	for i, node := range r.masterNodes {
		if node.Name == masterNode.Name {
			r.masterNodes[i] = masterNode
			return
		}
	}
	// Add the node to the embedded registry struct if it doesn't exist
	r.masterNodes = append(r.masterNodes, masterNode)
}

func (r *RegistryReconciler) SecretsStartupCheckCreate(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Embedded registry startup initialization", "component", "registry-controller")

	// Lock mutex to ensure thread safety
	r.embeddedRegistry.Mutex.Lock()
	defer r.embeddedRegistry.Mutex.Unlock()

	// Get the required environment variables
	registryAddress := os.Getenv("REGISTRY_ADDRESS")
	registryPath := os.Getenv("REGISTRY_PATH")
	imageDockerAuth := os.Getenv("IMAGE_DOCKER_AUTH")
	imageDockerDistribution := os.Getenv("IMAGE_DOCKER_DISTRIBUTION")

	if registryAddress == "" || imageDockerAuth == "" || imageDockerDistribution == "" || registryPath == "" {
		return fmt.Errorf("missing required environment variables: REGISTRY_ADDRESS, REGISTRY_PATH, IMAGE_DOCKER_AUTH, or IMAGE_DOCKER_DISTRIBUTION")
	}

	// Fill the embedded registry images struct with the registry address and image names
	r.embeddedRegistry.images.DockerAuth = fmt.Sprintf("%s%s@%s", registryAddress, registryPath, imageDockerAuth)
	r.embeddedRegistry.images.DockerDistribution = fmt.Sprintf("%s%s@%s", registryAddress, registryPath, imageDockerDistribution)

	// Ensure	 CA certificate exists and create if not
	isGenerated, caCertPEM, caKeyPEM, authTokenCertPEM, authTokenKeyPEM, err := k8s.EnsureCASecret(ctx, r.KubeClient)
	if err != nil {
		return err
	}

	// If CA certificate was generated, delete all PKI secrets
	if isGenerated {
		logger.Info("New registry root CA generated", "secret", "registry-pki", "component", "registry-controller")

		// Delete all PKI secrets
		deletedSecrets, err := k8s.DeleteAllRegistryNodeSecrets(ctx, r.KubeClient)
		if err != nil {
			return err
		}
		for _, deletedSecret := range deletedSecrets {
			logger.Info("Deleted node PKI secret, because CA certificate was regenerated", "secret", deletedSecret, "component", "registry-controller")
		}
	}

	// Fill the embedded registry struct with the CA PKI
	r.embeddedRegistry.caPKI = k8s.Certificate{
		Cert: caCertPEM,
		Key:  caKeyPEM,
	}

	// Fill the embedded registry struct with the Auth Token PKI
	r.embeddedRegistry.authTokenPKI = k8s.Certificate{
		Cert: authTokenCertPEM,
		Key:  authTokenKeyPEM,
	}

	masterNodes, err := k8s.GetMasterNodes(ctx, r.KubeClient)
	if err != nil {
		return err
	}

	for _, masterNode := range masterNodes {

		// Check if the node PKI secret exists
		secret, err := k8s.GetRegistryNodeSecret(ctx, r.KubeClient, masterNode.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Create the node PKI secret if it doesn't exist
		if secret == nil {
			dc, dk, ac, ak, err := k8s.CreateNodePKISecret(ctx, r.KubeClient, masterNode, caCertPEM, caKeyPEM)
			if err != nil {
				return err
			}
			logger.Info("Node secret created", "nodeName", masterNode.Name, "component", "registry-controller")

			masterNode.AuthCertificate = k8s.Certificate{
				Cert: ac,
				Key:  ak,
			}
			masterNode.DistributionCertificate = k8s.Certificate{
				Cert: dc,
				Key:  dk,
			}
		} else {
			// Extract the existing secret data
			masterNode.AuthCertificate = k8s.Certificate{
				Cert: secret.Data[k8s.AuthCert],
				Key:  secret.Data[k8s.AuthKey],
			}
			masterNode.DistributionCertificate = k8s.Certificate{
				Cert: secret.Data[k8s.DistributionCert],
				Key:  secret.Data[k8s.DistributionKey],
			}
		}

		// Add the node to the embedded registry struct
		r.embeddedRegistry.masterNodes = append(r.embeddedRegistry.masterNodes, masterNode)
	}

	// Ensure registry user secrets exist and create if not
	var registryUserRwSecret *k8s.RegistryUser
	var registryUserRoSecret *k8s.RegistryUser

	registryUserRwSecret, err = k8s.GetRegistryUser(ctx, r.KubeClient, "registry-user-rw")
	if err != nil {
		if apierrors.IsNotFound(err) {
			registryUserRwSecret, err = k8s.CreateRegistryUser(ctx, r.KubeClient, "registry-user-rw")
			logger.Info("Created registry rw user secret", "component", "registry-controller")
		} else {
			return err
		}
	}

	registryUserRoSecret, err = k8s.GetRegistryUser(ctx, r.KubeClient, "registry-user-ro")
	if err != nil {
		if apierrors.IsNotFound(err) {
			registryUserRoSecret, err = k8s.CreateRegistryUser(ctx, r.KubeClient, "registry-user-ro")
			logger.Info("Created registry ro user secret", "component", "registry-controller")
		} else {
			return err
		}
	}

	// Fill the embedded registry struct with the registry user secrets
	r.embeddedRegistry.registryRwUser = *registryUserRwSecret
	r.embeddedRegistry.registryRoUser = *registryUserRoSecret

	logger.Info("Embedded registry startup initialization complete", "component", "registry-controller")
	return nil
}

// extractModuleConfigFieldsFromObject extracts the 'enabled' and 'settings' fields from the ModuleConfig CR
func (r *RegistryReconciler) extractModuleConfigFieldsFromObject(cr *unstructured.Unstructured) (bool, map[string]interface{}, error) {
	// Extract the 'enabled' field from the ModuleConfig CR
	enabled, found, err := unstructured.NestedBool(cr.Object, "spec", "enabled")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'enabled' not found or failed to parse: %w", err)
	}
	// Extract the 'settings' field from the ModuleConfig CR
	settings, found, err := unstructured.NestedMap(cr.Object, "spec", "settings")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'settings' not found or failed to parse: %w", err)
	}
	return enabled, settings, nil
}
