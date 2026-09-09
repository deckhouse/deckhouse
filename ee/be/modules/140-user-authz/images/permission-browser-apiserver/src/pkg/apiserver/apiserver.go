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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/metrics"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/source"

	"permission-browser-apiserver/pkg/apis/authorization"
	"permission-browser-apiserver/pkg/apis/authorization/install"
	"permission-browser-apiserver/pkg/authorizer/composite"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
	"permission-browser-apiserver/pkg/authorizer/scopefilter"
	"permission-browser-apiserver/pkg/registry"
	"permission-browser-apiserver/pkg/resolver"
)

const (
	// metricsNamespace prefixes the rules metrics of this apiserver; the webhook uses its own, so
	// the two consumers can be compared side by side.
	metricsNamespace = "user_authz_permission_browser"

	// metricsListenAddr is a plaintext listener that serves only /metrics. A kube-rbac-proxy
	// sidecar fronts it on the pod IP for Prometheus, so the aggregated API server's own port stays
	// behind the aggregation layer. The address is the loopback one, so nothing outside the Pod
	// reaches it directly.
	metricsListenAddr = "127.0.0.1:4276"
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

// ExtraConfig holds custom apiserver config. The multi-tenancy rules are read from the
// ClusterAuthorizationRules of the cluster, so there is nothing to configure at the moment.
type ExtraConfig struct{}

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
	dynamicClient   dynamic.Interface
	informerFactory informers.SharedInformerFactory
	restConfig      *rest.Config
}

// initInformers initializes the Kubernetes clients and the shared informer factory.
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

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	result.dynamicClient = dynamicClient

	// Create shared informer factory with 30 minute resync
	result.informerFactory = informers.NewSharedInformerFactory(clientset, 30*time.Minute)

	return result, nil
}

// rulesInputs are the two feeds of the multi-tenancy engine: the rules themselves and the index of
// the ClusterRoleBindings user-authz-controller created for them.
type rulesInputs struct {
	rules    *source.Source
	bindings *binding.Index
	// bindingsSynced reports that the index has actually been fed. The informer reports synced once
	// the initial list has been popped, while handlers are fed from a separate queue, so a report
	// built on the informer's word alone could miss the bindings that make a subject restricted.
	bindingsSynced kcache.InformerSynced
	registry       *prometheus.Registry
}

// serveMetrics runs the plaintext /metrics listener until the context is cancelled. A failure to
// listen is logged, not fatal: the apiserver answers requests whether or not its metrics are
// scraped.
func (in *rulesInputs) serveMetrics(ctx context.Context) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(in.registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:         metricsListenAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Warningf("metrics listener on %s stopped: %v", metricsListenAddr, err)
	}
}

// initRules wires the rules feeds. The bindings index is fed from the same ClusterRoleBinding
// informer the RBAC authorizer uses; the rules come from their own informer on
// ClusterAuthorizationRules, run in the background from New.
func initRules(init *initResult) (*rulesInputs, error) {
	bindings := binding.NewIndex()
	bindingsSynced, err := init.informerFactory.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(bindings.EventHandler())
	if err != nil {
		return nil, fmt.Errorf("register rule bindings index: %w", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	rulesMetrics := metrics.New(metricsNamespace)
	if err := rulesMetrics.Register(registry); err != nil {
		return nil, fmt.Errorf("register rules metrics: %w", err)
	}

	rules := source.New(init.dynamicClient, source.Options{
		Observer: rulesMetrics,
		Logf:     klog.Infof,
	})
	return &rulesInputs{
		rules:          rules,
		bindings:       bindings,
		bindingsSynced: bindingsSynced.HasSynced,
		registry:       registry,
	}, nil
}

// initAuthorizers creates the composite authorizer from RBAC and multi-tenancy engines.
func initAuthorizers(init *initResult, inputs *rulesInputs, scopeCache *resolver.ResourceScopeCache) (authorizer.Authorizer, *multitenancy.Engine, error) {
	if init.informerFactory == nil {
		return nil, nil, fmt.Errorf("informer factory is not available, cannot initialize authorizers")
	}

	// Left nil when discovery is unavailable, which turns the identity-read
	// reporting off rather than letting it guess.
	var registry scopefilter.ResourceRegistry
	if scopeCache != nil {
		registry = scopeCache
	}

	// Create RBAC authorizer
	rbacAuth := rbacadapter.NewRBACAuthorizer(init.informerFactory)

	// Same guard for the engine: a nil *ResourceScopeCache must arrive as a nil
	// interface, not as a non-nil interface holding a nil pointer.
	var resourceScope multitenancy.ResourceScope
	if scopeCache != nil {
		resourceScope = scopeCache
	}

	// Create multi-tenancy engine
	var mtEngine *multitenancy.Engine
	if inputs != nil {
		var err error
		mtEngine, err = multitenancy.NewEngine(
			inputs.rules,
			inputs.bindings,
			init.informerFactory.Core().V1().Namespaces().Lister(),
			init.informerFactory.Core().V1().Namespaces().Informer().HasSynced,
			resourceScope,
		)
		if err != nil {
			klog.Warningf("Failed to initialize multi-tenancy engine: %v. Multi-tenancy restrictions will not be applied.", err)
			mtEngine = nil
		}
	}

	// Combine authorizers. The outermost layer reports reads that the apiserver
	// answers from the namespace ACL rather than from RBAC, which no amount of
	// RBAC analysis below it can discover.
	if mtEngine != nil {
		// Requests granted by CAR-independent RBAC (RoleBindings, non-CAR
		// ClusterRoleBindings) must not be denied by multi-tenancy filters.
		mtEngine.SetIndependentRBACChecker(rbacAuth)
		return scopefilter.NewIdentityReadAuthorizer(composite.NewCompositeAuthorizer(mtEngine, rbacAuth), registry), mtEngine, nil
	}
	return scopefilter.NewIdentityReadAuthorizer(rbacAuth, registry), nil, nil
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

	// Create resource scope cache for background discovery refresh. The
	// authorizers below consult it, so it has to exist before them.
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

	// The multi-tenancy rules and the bindings that point at them. Registered before the informer
	// factory starts so the bindings index sees the initial list.
	var inputs *rulesInputs
	if initRes.informerFactory != nil && initRes.dynamicClient != nil {
		inputs, err = initRules(initRes)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to initialize the rules feeds: %w", err)
		}

		// A report built before the rules were listed once would show every subject of a rule as
		// maximally restricted. Readiness waits for the list; a cluster without the CRD is ready,
		// there are no rules to wait for.
		if err := genericServer.AddReadyzChecks(healthz.NamedCheck("user-authz-rules", func(_ *http.Request) error {
			if !inputs.bindingsSynced() {
				return fmt.Errorf("the ClusterRoleBindings of the rules are not indexed yet")
			}
			if state := inputs.rules.State(); state == source.StateUnsynced {
				if lastErr := inputs.rules.LastError(); lastErr != nil {
					return fmt.Errorf("ClusterAuthorizationRules are not listed yet: %v", lastErr)
				}
				return fmt.Errorf("ClusterAuthorizationRules are not listed yet")
			}
			return nil
		})); err != nil {
			klog.Warningf("Failed to add user-authz-rules readyz check: %v", err)
		}
	}

	// Initialize authorizers
	compositeAuth, mtEngine, err := initAuthorizers(initRes, inputs, scopeCache)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize authorizers: %w", err)
	}

	// Start informers
	startInformers(ctx, initRes.informerFactory)

	// Start the rules informer. It is not part of the caches waited for above: at bootstrap the CRD
	// may not exist yet, and until the rules are listed the bindings index keeps every subject of a
	// rule maximally restricted.
	if inputs != nil {
		go inputs.rules.Run(ctx)
		go inputs.serveMetrics(ctx)
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
