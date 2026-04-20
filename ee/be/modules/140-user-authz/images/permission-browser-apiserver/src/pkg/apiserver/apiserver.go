/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization"
	"permission-browser-apiserver/pkg/apis/authorization/install"
	"permission-browser-apiserver/pkg/authorizer/composite"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
	"permission-browser-apiserver/pkg/registry"
	"permission-browser-apiserver/pkg/resolver"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(Scheme)

	// we need to add the options to empty v1
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	// ConfigPath is the path to the user-authz-webhook config file
	ConfigPath string
}

// Config defines the config for the apiserver
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// PermissionBrowserServer contains state for a Kubernetes cluster master/api server.
type PermissionBrowserServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	return CompletedConfig{&c}
}

// initResult holds the initialization results
type initResult struct {
	clientset       *kubernetes.Clientset
	informerFactory informers.SharedInformerFactory
	restConfig      *rest.Config
}

// initInformers initializes the Kubernetes client and shared informer factory.
func initInformers() (*initResult, error) {
	result := &initResult{}

	// Initialize Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Warningf("Failed to get in-cluster config: %v", err)
		return result, nil
	}
	result.restConfig = config

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	result.clientset = clientset

	// Create shared informer factory with 30 minute resync
	result.informerFactory = informers.NewSharedInformerFactory(clientset, 30*time.Minute)

	return result, nil
}

// initAuthorizers creates the composite authorizer from RBAC and multi-tenancy engines.
func initAuthorizers(init *initResult, configPath string) (authorizer.Authorizer, *multitenancy.Engine, error) {
	if init.informerFactory == nil {
		return nil, nil, fmt.Errorf("informer factory is not available, cannot initialize authorizers")
	}

	// Create RBAC authorizer
	rbacAuth := rbacadapter.NewRBACAuthorizer(init.informerFactory)

	// Resolve config path
	if configPath == "" {
		configPath = "/etc/user-authz-webhook/config.json"
	}

	// Create multi-tenancy engine
	var mtEngine *multitenancy.Engine
	if init.clientset != nil {
		var err error
		mtEngine, err = multitenancy.NewEngine(
			configPath,
			init.informerFactory.Core().V1().Namespaces().Lister(),
			init.informerFactory.Core().V1().Namespaces().Informer().HasSynced,
			init.clientset.Discovery(),
		)
		if err != nil {
			klog.Warningf("Failed to initialize multi-tenancy engine: %v. Multi-tenancy restrictions will not be applied.", err)
			mtEngine = nil
		}
	}

	// Combine authorizers
	if mtEngine != nil {
		return composite.NewCompositeAuthorizer(mtEngine, rbacAuth), mtEngine, nil
	}
	return rbacAuth, nil, nil
}

// startInformers starts the informer factory and waits for cache sync.
func startInformers(ctx context.Context, informerFactory informers.SharedInformerFactory) {
	if informerFactory == nil {
		return
	}

	informerFactory.Start(ctx.Done())

	klog.Info("Waiting for informer caches to sync...")
	informerFactory.WaitForCacheSync(ctx.Done())
	klog.Info("Informer caches synced")
}

// registerAPIGroup registers the authorization API group with the server.
func registerAPIGroup(server *genericapiserver.GenericAPIServer, auth authorizer.Authorizer, nsResolver *resolver.NamespaceResolver) error {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		authorization.GroupName,
		Scheme,
		metav1.ParameterCodec,
		Codecs,
	)

	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = registry.GetStorage(auth)
	if nsResolver != nil {
		apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = registry.GetStorageWithResolver(auth, nsResolver)
	}

	return server.InstallAPIGroup(&apiGroupInfo)
}

// New returns a new instance of PermissionBrowserServer from the given config.
func (c completedConfig) New() (*PermissionBrowserServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create generic API server
	genericServer, err := c.GenericConfig.New("permission-browser-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		cancel()
		return nil, err
	}

	// Register shutdown hook to cancel context
	if err := genericServer.AddPreShutdownHook("cancel-context", func() error {
		cancel()
		return nil
	}); err != nil {
		cancel()
		return nil, err
	}

	// Initialize informers
	initRes, err := initInformers()
	if err != nil {
		cancel()
		return nil, err
	}

	// Initialize authorizers
	compositeAuth, mtEngine, err := initAuthorizers(initRes, c.ExtraConfig.ConfigPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize authorizers: %w", err)
	}

	// Start informers
	startInformers(ctx, initRes.informerFactory)

	// Start multi-tenancy config renewal
	if mtEngine != nil {
		go mtEngine.StartRenewConfigLoop(ctx.Done())
	}

	// Create resource scope cache for background discovery refresh
	var scopeCache *resolver.ResourceScopeCache
	if initRes.clientset != nil {
		scopeCache = resolver.NewResourceScopeCache(initRes.clientset.Discovery())
		go scopeCache.StartRefreshLoop(ctx.Done())
		klog.Info("Resource scope cache initialized and refresh loop started")

		// Ensure readiness fails until the cache has been populated at least once.
		if err := genericServer.AddReadyzChecks(healthz.NamedCheck("resource-scope-cache", func(_ *http.Request) error {
			if !scopeCache.HasData() {
				return fmt.Errorf("resource scope cache is empty")
			}
			return nil
		})); err != nil {
			klog.Warningf("Failed to add resource-scope-cache readyz check: %v", err)
		}
	}

	// Create namespace resolver for AccessibleNamespace API
	var nsResolver *resolver.NamespaceResolver
	if initRes.informerFactory != nil {
		rbacInformers := initRes.informerFactory.Rbac().V1()
		nsResolver = resolver.NewNamespaceResolver(
			initRes.informerFactory.Core().V1().Namespaces().Lister(),
			rbacInformers.Roles().Lister(),
			rbacInformers.RoleBindings().Lister(),
			rbacInformers.ClusterRoles().Lister(),
			rbacInformers.ClusterRoleBindings().Lister(),
			scopeCache,
			mtEngine,
		)
		klog.Info("Namespace resolver initialized for AccessibleNamespace API")
	}

	// Register API group
	if err := registerAPIGroup(genericServer, compositeAuth, nsResolver); err != nil {
		cancel()
		return nil, err
	}

	return &PermissionBrowserServer{
		GenericAPIServer: genericServer,
	}, nil
}
