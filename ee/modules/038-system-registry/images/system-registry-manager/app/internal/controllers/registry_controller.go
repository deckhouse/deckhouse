package controllers

import (
	"context"
	httpclient "embeded-registry-manager/internal/utils/http_client"
	"embeded-registry-manager/internal/utils/k8s"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"sync"
)

type ModuleConfig struct {
	enabled  bool
	settings map[string]interface{}
}

type embeddedRegistry struct {
	rwMutex        sync.RWMutex
	mc             ModuleConfig
	CaPKI          k8s.Certificate
	RegistryRwUser k8s.RegistryUser
	RegistryRoUser k8s.RegistryUser
	MasterNodes    []k8s.MasterNode
}

type RegistryReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	KubeClient        *kubernetes.Clientset
	Recorder          record.EventRecorder
	HttpClient        *httpclient.Client
	nonCacheApiReader client.Reader
	embeddedRegistry  embeddedRegistry
}

var nodePKISecretRegex = regexp.MustCompile(`^registry-node-.*-pki$`)

// SetupWithManager sets up the controller with the Manager to watch for changes in both Pods and Secrets.
func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {

	// Lock the mutex for the embedded registry struct to prevent simultaneous writes
	r.embeddedRegistry.rwMutex.Lock()
	defer r.embeddedRegistry.rwMutex.Unlock()

	// Set the nonCacheApiReader to be used in functions before the cache is initialized
	r.nonCacheApiReader = mgr.GetAPIReader()

	// Extract ModuleConfig fields, generate error if not found, disabled or failed to parse
	enabled, settings, err := r.extractModuleConfigFields(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("ModuleConfig not found, EmbeddedRegistry should be disabled. Error: %w", err)
		} else if enabled == false {
			return fmt.Errorf("ModuleConfig not enabled, EmbeddedRegistry should be disabled: %w", err)
		} else {
			return fmt.Errorf("error retrieving fields from ModuleConfig: %w", err)

		}
	}
	r.embeddedRegistry.mc.enabled = enabled
	r.embeddedRegistry.mc.settings = settings

	// Check and create certificates on startup
	err = r.secretsStartupCheckCreate(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize certs: %w", err)
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

func (r *RegistryReconciler) secretEventHandler() handler.EventHandler {
	secretsToWatch := []string{
		"registry-user-ro",
		"registry-user-rw",
		"registry-pki",
	}

	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		secretName := obj.GetName()
		//elog := ctrl.LoggerFrom(ctx)

		// Helper function to enqueue reconcile request
		enqueue := func(name, namespace string) []reconcile.Request {
			//elog.Info("Enqueuing reconcile request for secret", "secret", name)
			return []reconcile.Request{
				{NamespacedName: client.ObjectKey{
					Name:      name,
					Namespace: namespace,
				}},
			}
		}

		// Check if the secret name matches the list
		for _, currentSecretName := range secretsToWatch {
			if secretName == currentSecretName {
				return enqueue(obj.GetName(), obj.GetNamespace())
			}
		}

		// Check for the "registry-node-*-pki" pattern
		if strings.HasPrefix(secretName, "registry-node-") && strings.HasSuffix(secretName, "-pki") {
			return enqueue(obj.GetName(), obj.GetNamespace())
		}

		return nil
	})
}

func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Lock the mutex for the embedded registry struct to prevent simultaneous writes
	r.embeddedRegistry.rwMutex.Lock()
	defer r.embeddedRegistry.rwMutex.Unlock()

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)

	//logger.Info("embeddedRegistry.MasterNodes struct", "struct", r.embeddedRegistry.MasterNodes)

	switch {
	// Check if the secret is the registry-pki secret
	case req.NamespacedName.Name == "registry-pki":
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Registry PKI was deleted", "Secret Name", req.NamespacedName.Name)
				_, caCertPEM, caKeyPEM, err := k8s.EnsureCASecret(ctx, r.KubeClient)
				//
				if err != nil {
					return ctrl.Result{}, err
				}
				r.embeddedRegistry.CaPKI = k8s.Certificate{
					Cert: caCertPEM,
					Key:  caKeyPEM,
				}
				logger.Info("New Registry root CA generated")
			} else {
				//
				return ctrl.Result{}, err
			}
		} else {
			// Check if the CA certificate has changed
			if string(secret.Data[k8s.RegistryCACert]) == string(r.embeddedRegistry.CaPKI.Cert) {
				logger.Info("Registry CA not changed")
				return ctrl.Result{}, nil
			}
			logger.Info("Registry CA changed")
			r.embeddedRegistry.CaPKI = k8s.Certificate{
				Cert: secret.Data[k8s.RegistryCACert],
				Key:  secret.Data[k8s.RegistryCAKey],
			}
		}

		// Clear the master nodes slice
		r.embeddedRegistry.MasterNodes = nil

		// Delete all PKI secrets
		deletedSecrets, err := k8s.DeleteAllRegistryNodeSecrets(ctx, r.KubeClient)
		if err != nil {
			return ctrl.Result{}, err
		}
		for _, deletedSecret := range deletedSecrets {
			logger.Info("Registry node PKI deleted secret due to CA certificate change", "secret", deletedSecret)
		}

		return ctrl.Result{}, nil
	// Check if the secret is the registry-node-*-pki secret
	case nodePKISecretRegex.MatchString(req.NamespacedName.Name):
		nodeName := strings.TrimPrefix(strings.TrimSuffix(req.NamespacedName.Name, "-pki"), "registry-node-")

		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Node PKI secret was deleted", "Secret Name", req.NamespacedName.Name)
				err := r.recreateNodePKISecret(ctx, nodeName)
				if err != nil {
					return ctrl.Result{}, err
				} else {
					// TODO ???
					return ctrl.Result{}, nil
				}
			} else {
				return ctrl.Result{}, err
			}
		}

		// Check if the secret is the registry-node-*-pki secret
		nodeChanged, err := r.checkAndUpdateNodePKISecret(ctx, secret, nodeName)
		if err != nil {
			return ctrl.Result{}, err
		} else if !nodeChanged {
			return ctrl.Result{}, nil
		} else {
			// TODO ???
			return ctrl.Result{}, nil
		}

	// Check if the secret is the registry-user-rw secret
	case req.NamespacedName.Name == "registry-user-rw":
		secretChanged, err := r.reconcileRegistryUser(ctx, req, "registry-user-rw", &r.embeddedRegistry.RegistryRwUser)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !secretChanged {
			return ctrl.Result{}, nil
		} else {
			// TODO do not return, but pass to the end of the function to call static pod update api
			return ctrl.Result{}, nil
		}
	// Check if the secret is the registry-user-ro secret
	case req.NamespacedName.Name == "registry-user-ro":
		secretChanged, err := r.reconcileRegistryUser(ctx, req, "registry-user-ro", &r.embeddedRegistry.RegistryRoUser)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !secretChanged {
			return ctrl.Result{}, nil
		} else {
			// TODO do not return, but pass to the end of the function to call static pod update api
			return ctrl.Result{}, nil
		}
	}

	logger.Info("UNHANDLED event", "secret", req.NamespacedName.Name)

	return ctrl.Result{}, nil
}

func (r *RegistryReconciler) recreateNodePKISecret(ctx context.Context, nodeName string) error {
	logger := ctrl.LoggerFrom(ctx)

	// Get the master node by name
	masterNode, err := k8s.GetMasterNodeByName(ctx, r.KubeClient, nodeName)
	if err != nil {
		return err
	}

	// Create the node PKI secret
	dc, dk, ac, ak, err := k8s.CreateNodePKISecret(ctx, r.KubeClient, masterNode, r.embeddedRegistry.CaPKI.Cert, r.embeddedRegistry.CaPKI.Key)
	if err != nil {
		return err
	}

	// Fill the master node struct with the certificates
	masterNode.DistributionCertificate = k8s.Certificate{Cert: dc, Key: dk}
	masterNode.AuthCertificate = k8s.Certificate{Cert: ac, Key: ak}

	// Add the node to the embedded registry struct
	r.embeddedRegistry.updateMasterNode(masterNode)

	logger.Info("Node secret recreated for node", "nodeName", masterNode.Name)
	return nil
}

func (r *RegistryReconciler) checkAndUpdateNodePKISecret(ctx context.Context, secret *corev1.Secret, nodeName string) (bool, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Get the master node by name
	masterNode, found := r.embeddedRegistry.getMasterNodeFromEmbeddedRegistryStruct(nodeName)
	if !found {
		return false, fmt.Errorf("master node %s not found in embeddedRegistry", nodeName)
	}

	// Check if the node PKI secret has changed
	if isNodePKISecretUpToDate(secret, masterNode) {
		logger.Info("Registry Node PKI not changed", "Secret Name", secret.Name)
		return false, nil
	}

	// If the secret has changed, update the master node struct
	masterNode.AuthCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.AuthCert],
		Key:  secret.Data[k8s.AuthKey],
	}
	masterNode.DistributionCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.DistributionCert],
		Key:  secret.Data[k8s.DistributionKey],
	}

	logger.Info("Registry Node PKI changed", "node name", masterNode.Name, "Secret Name", secret.Name)
	return true, nil
}

func (r *embeddedRegistry) getMasterNodeFromEmbeddedRegistryStruct(nodeName string) (*k8s.MasterNode, bool) {
	for i, node := range r.MasterNodes {
		if node.Name == nodeName {
			return &r.MasterNodes[i], true
		}
	}
	return nil, false
}

func (r *embeddedRegistry) updateMasterNode(masterNode k8s.MasterNode) {
	// Update the node in the embedded registry struct if it exists
	for i, node := range r.MasterNodes {
		if node.Name == masterNode.Name {
			r.MasterNodes[i] = masterNode
			return
		}
	}
	// Add the node to the embedded registry struct if it doesn't exist
	r.MasterNodes = append(r.MasterNodes, masterNode)
}

func isNodePKISecretUpToDate(secret *corev1.Secret, masterNode *k8s.MasterNode) bool {
	return string(secret.Data[k8s.AuthCert]) == string(masterNode.AuthCertificate.Cert) &&
		string(secret.Data[k8s.AuthKey]) == string(masterNode.AuthCertificate.Key) &&
		string(secret.Data[k8s.DistributionCert]) == string(masterNode.DistributionCertificate.Cert) &&
		string(secret.Data[k8s.DistributionKey]) == string(masterNode.DistributionCertificate.Key)
}

func (r *RegistryReconciler) reconcileRegistryUser(ctx context.Context, req ctrl.Request, secretName string, user *k8s.RegistryUser) (bool, error) {
	logger := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the registry user secret if it doesn't exist
			newUserSecret, err := k8s.CreateRegistryUser(ctx, r.KubeClient, secretName)
			if err != nil {
				return false, err
			}
			*user = *newUserSecret
			logger.Info("Created registry user secret", "secretName", secretName)
			return true, nil
		} else {
			// Return the error if other error occurred
			return false, err
		}
	}

	// Check if the registry user secret has changed
	if string(secret.Data["name"]) == user.UserName &&
		string(secret.Data["password"]) == user.Password &&
		string(secret.Data["passwordHash"]) == user.HashedPassword {
		logger.Info("Registry user password not changed", "Secret Name", req.NamespacedName.Name)
		return false, nil
	} else {
		// If the secret has changed, update the user struct
		user.UserName = string(secret.Data["name"])
		user.Password = string(secret.Data["password"])
		user.HashedPassword = string(secret.Data["passwordHash"])
		logger.Info("Registry user password changed", "Secret Name", req.NamespacedName.Name)
		return true, nil
	}
}

func (r *RegistryReconciler) secretsStartupCheckCreate(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Embedded registry startup certificates check", "component", "registry-controller")

	// Ensure CA certificate exists and create if not
	isGenerated, caCertPEM, caKeyPEM, err := k8s.EnsureCASecret(ctx, r.KubeClient)
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
	r.embeddedRegistry.CaPKI = k8s.Certificate{
		Cert: caCertPEM,
		Key:  caKeyPEM,
	}

	masterNodes, err := k8s.GetMasterNodes(ctx, r.KubeClient)
	if err != nil {
		return err
	}

	nodesPKISecrets, err := k8s.GetAllRegistryNodeSecrets(ctx, r.KubeClient)
	if err != nil {
		return err
	}

	secretDataMap := make(map[string]k8s.NodeSecretData)
	for _, secret := range nodesPKISecrets {
		data := k8s.NodeSecretData{
			AuthCrt:         secret.Data[k8s.AuthCert],
			AuthKey:         secret.Data[k8s.AuthKey],
			DistributionCrt: secret.Data[k8s.DistributionCert],
			DistributionKey: secret.Data[k8s.DistributionKey],
		}
		secretDataMap[secret.Name] = data
	}

	for _, masterNode := range masterNodes {
		nodeSecretName := fmt.Sprintf("registry-node-%s-pki", masterNode.Name)

		// Create the node PKI secret if it doesn't exist
		if _, exists := secretDataMap[nodeSecretName]; !exists {

			if dc, dk, ac, ak, err := k8s.CreateNodePKISecret(ctx, r.KubeClient, masterNode, caCertPEM, caKeyPEM); err != nil {
				return err
			} else {
				secretDataMap[nodeSecretName] = k8s.NodeSecretData{
					AuthCrt:         ac,
					AuthKey:         ak,
					DistributionCrt: dc,
					DistributionKey: dk,
				}
			}
			logger.Info("Node secret created for node", "nodeName", masterNode.Name, "component", "registry-controller")
		}

		// Fill the master node struct with the certificates
		secretData := secretDataMap[nodeSecretName]
		masterNode.AuthCertificate = k8s.Certificate{
			Cert: secretData.AuthCrt,
			Key:  secretData.AuthKey,
		}
		masterNode.DistributionCertificate = k8s.Certificate{
			Cert: secretData.DistributionCrt,
			Key:  secretData.DistributionKey,
		}

		// Add the node to the embedded registry struct
		r.embeddedRegistry.MasterNodes = append(r.embeddedRegistry.MasterNodes, masterNode)
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
	r.embeddedRegistry.RegistryRwUser = *registryUserRwSecret
	r.embeddedRegistry.RegistryRoUser = *registryUserRoSecret

	return nil
}

func (r *RegistryReconciler) extractModuleConfigFields(ctx context.Context) (bool, map[string]interface{}, error) {
	cr := &unstructured.Unstructured{}
	cr.SetAPIVersion(k8s.ModuleConfigApiVersion)
	cr.SetKind(k8s.ModuleConfigKind)

	//
	err := r.nonCacheApiReader.Get(ctx, client.ObjectKey{Namespace: k8s.RegistryNamespace, Name: k8s.RegistryMcName}, cr)
	if err != nil {
		return false, nil, err
	}

	//
	enabled, found, err := unstructured.NestedBool(cr.Object, "spec", "enabled")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'enabled' not found or failed to parse: %w", err)
	}

	//
	settings, found, err := unstructured.NestedMap(cr.Object, "spec", "settings")
	if err != nil || !found {
		return false, nil, fmt.Errorf("field 'settings' not found or failed to parse: %w", err)
	}

	return enabled, settings, nil
}
