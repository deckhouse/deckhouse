/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha1"
	deckhousev1alpha2 "controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	namespacecontroller "controller/internal/controller/namespace"
	projectcontroller "controller/internal/controller/project"
	templatecontroller "controller/internal/controller/template"
	grantcontrollers "controller/internal/controllers"
	clusterprojectrolebindingcontroller "controller/internal/controllers/clusterprojectrolebinding"
	projectgrantscontroller "controller/internal/controllers/projectgrants"
	projectnamescontroller "controller/internal/controllers/projectnames"
	projectnamespacecontroller "controller/internal/controllers/projectnamespace"
	projectquotacontroller "controller/internal/controllers/projectquota"
	projectrolebindingcontroller "controller/internal/controllers/projectrolebinding"
	templategrantscontroller "controller/internal/controllers/templategrants"
	"controller/internal/edition"
	"controller/internal/grantcache"
	"controller/internal/helm"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/quota"
	"controller/internal/quota/evaluator"
	"controller/internal/quota/ledger"
	"controller/internal/resolve"
	"controller/internal/rolebinding"
	"controller/internal/startup"
	clusterprojectrolebindingwebhook "controller/internal/webhook/clusterprojectrolebinding"
	grantableclusterresourcedefinitionwebhook "controller/internal/webhook/grantableclusterresourcedefinition"
	grantableclusterresourcereferencewebhook "controller/internal/webhook/grantableclusterresourcereference"
	projectwebhook "controller/internal/webhook/project"
	projectnamespacewebhook "controller/internal/webhook/projectnamespace"
	projectquotawebhook "controller/internal/webhook/projectquota"
	projectrolebindingwebhook "controller/internal/webhook/projectrolebinding"
	templatewebhook "controller/internal/webhook/template"
	grantwebhooks "controller/internal/webhooks"
)

var (
	// path to helm templates
	helmTemplatesPath = "helmlib"
	// path to default project templates
	templatesPath = "templates"
	// convertedCRDs are the CRDs the controller reads that the module converts with its ConversionWebhook
	// resources (templates/*-conversion-webhook.yaml).
	convertedCRDs = []string{
		"clusterresourcegrantpolicies.multitenancy.deckhouse.io",
		"projects.deckhouse.io",
		"projecttemplates.deckhouse.io",
	}
	// helm release namespace (same value Helm later writes on adopted objects)
	helmNamespace = helm.ReleaseStorageNamespace
	// controller service account (centralized in internal/rolebinding so the value cannot drift)
	serviceAccount = rolebinding.ControllerServiceAccount
)

const (
	controllerName = "multitenancy-manager"

	// Leader election is always on: even a single-replica Deployment has two pods during a rolling
	// update, and two writers of the same Helm releases would each upgrade them (the 2026-09-14
	// rollout left every project on the changed template with an extra, identical revision). The
	// webhooks keep serving on every replica; only the controllers wait for the lease.
	//
	// The lease is renewed against the API server, and controller-runtime exits the process when a
	// renewal misses RenewDeadline. With the defaults (15 s lease, 10 s renew) every short API-server
	// absence would restart the pod and throw away a warm cache; these give it a minute to come back,
	// the same budget user-authz-controller runs with.
	leaseDuration = 60 * time.Second
	renewDeadline = 40 * time.Second
	retryPeriod   = 8 * time.Second

	// podNameEnv names this pod; the Deployment sets it from the downward API.
	podNameEnv = "POD_NAME"

	// The controllers wait for the pods of the other revisions of the Deployment to finish, checking
	// every previousPodsInterval. The wait is bounded by previousPodsTimeout, so a pod stuck
	// terminating on a lost node does not keep them down for good.
	previousPodsInterval = 5 * time.Second
	previousPodsTimeout  = 10 * time.Minute

	// The controller process waits for the conversion webhook of convertedCRDs, checking every
	// crdConversionInterval; see startup.CRDConversion.
	crdConversionInterval = 2 * time.Second

	// healthProbeAddress serves the probes of the kubelet: by the manager, and before the manager is
	// created by startup.ServeProbesWhile.
	healthProbeAddress = ":9090"

	// logLevelEnv selects the zap level (debug, info, warn, error); the default is info, as in
	// user-authz-controller.
	logLevelEnv     = "LOG_LEVEL"
	defaultLogLevel = "info"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == quotaLedgerMode {
		runQuotaLedger()
		return
	}

	// setup logger
	logger := ctrl.Log.WithName(controllerName)
	setupLogging()

	// The edition is built into the binary (internal/edition): in Deckhouse Platform Open a project has
	// only its main namespace.
	singleNamespace := edition.SingleNamespaceProjects(edition.Current())
	logger.Info("start multitenancy-manager", "edition", edition.Current(), "singleNamespaceProjects", singleNamespace)

	ctx := ctrl.SetupSignalHandler()

	// Wait for the conversion webhook before the manager exists: its caches, controllers and webhooks
	// would read the objects of a CRD that converts with strategy None and keep that reading. The wait
	// is in the controller and not in a hook before the Helm release of the module, because that
	// release carries the ConversionWebhook resources the conversion comes from.
	if err := waitForCRDConversion(ctx, logger); err != nil {
		if ctx.Err() != nil {
			logger.Info("stopped while waiting for the conversion of the CRDs")
			return
		}
		fatal(logger, err, "wait for the conversion of the CRDs")
	}

	// The watch error handler of the manager's cache: a kind of the grant model whose list or watch
	// fails is read by the webhooks from the API server instead of a cache that stopped following it.
	grantModelWatch := grantwebhooks.NewModelWatch()

	// initialize runtime manager
	runtimeManager, err := setupRuntimeManager(logger, grantModelWatch.WatchError)
	if err != nil {
		fatal(logger, err, "set up runtime manager")
	}

	// Hold the controllers and the Init runnables registered below until no pod of another revision
	// of the Deployment is left. The lease keeps apart only the pods that take it; a pod of a
	// revision that does not would keep writing the Helm releases of the projects next to this one.
	// The hold starts once this replica holds the lease, while the webhooks already serve. The
	// Deployment runs in the namespace of the Helm release storage and of the lease.
	previousPods := &startup.PreviousPods{
		Reader:    runtimeManager.GetAPIReader(),
		Recorder:  runtimeManager.GetEventRecorderFor(controllerName),
		Logger:    logger.WithName("previous-pods"),
		Namespace: helmNamespace,
		Name:      os.Getenv(podNameEnv),
		Selector:  map[string]string{"app": controllerName},
		Interval:  previousPodsInterval,
		Timeout:   previousPodsTimeout,
	}
	if runtimeManager, err = startup.HoldControllers(runtimeManager, previousPods.Wait); err != nil {
		fatal(logger, err, "hold the controllers")
	}

	// initialize helm client
	helmClient, err := helm.New(helmNamespace, helmTemplatesPath, logger)
	if err != nil {
		fatal(logger, err, "initialize helm client")
	}
	// In an edition where a project has only its main namespace, an upgrade leaves the objects of the
	// namespaces a project dropped as they are (namespaces.Dropped).
	helmClient.SetDroppedNamespaces(func(ctx context.Context, project string) (map[string]struct{}, error) {
		return namespaces.Dropped(ctx, runtimeManager.GetClient(), project)
	})

	migration := startup.NewMigration()

	// register project controller
	if err = projectcontroller.Register(runtimeManager, helmClient, logger, migration); err != nil {
		fatal(logger, err, "register project controller")
	}

	// register template controller
	if err = templatecontroller.Register(runtimeManager, templatesPath, logger); err != nil {
		fatal(logger, err, "register template controller")
	}

	// register the schema-based template grant materializer: turns a v1alpha2 template's
	// resources/grantPolicies into managed ClusterResourceGrantPolicy objects.
	if err = (&templategrantscontroller.Reconciler{
		Client: runtimeManager.GetClient(),
		Scheme: runtimeManager.GetScheme(),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "register template grants reconciler")
	}

	// register the project grant materializer: turns spec.grants of a project into managed
	// ClusterResourceGrantPolicy objects.
	if err = (&projectgrantscontroller.Reconciler{
		Client:   runtimeManager.GetClient(),
		Scheme:   runtimeManager.GetScheme(),
		Recorder: runtimeManager.GetEventRecorderFor(controllerName),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "register project grants reconciler")
	}

	// register namespace controller
	if err = namespacecontroller.Register(runtimeManager, logger, migration); err != nil {
		fatal(logger, err, "register namespace controller")
	}

	// The keys of a project quota that count objects of a resource are checked against the discovery
	// of the API server; the cache of the webhooks is invalidated when a resource is not found, so a CRD
	// installed a moment ago is seen. The controller has a cache of its own, invalidated at most once in
	// a few seconds: a quota with a key of a group not served yet misses on every reconcile.
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(runtimeManager.GetConfig(), runtimeManager.GetHTTPClient())
	if err != nil {
		fatal(logger, err, "create discovery client")
	}
	quotaKeys := &quota.APIDiscovery{Client: memory.NewMemCacheClient(discoveryClient)}
	reconcileQuotaKeys := &quota.APIDiscovery{Client: memory.NewMemCacheClient(discoveryClient), InvalidateEvery: quota.ReconcileInvalidateEvery}
	quotaConfig := &quota.ConfigReader{Reader: runtimeManager.GetAPIReader()}

	// register project webhook
	projectwebhook.Register(runtimeManager, helmClient, quotaKeys)
	// register the webhook that lets a project go only with the right to delete its namespaces
	projectwebhook.RegisterDelete(runtimeManager)

	// register template webhook
	templatewebhook.Register(runtimeManager, serviceAccount)

	// register cluster resource grants: catalog reconciler, binding-status reconcilers and webhooks.
	// They get their own REST mapper, reset every ResyncInterval and on a list 404 of a granted
	// kind, so a removed or re-scoped CRD is noticed; the manager's mapper never forgets a mapping.
	grantsMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	if err = runtimeManager.Add(&grantcontrollers.MapperReset{Mapper: grantsMapper, Interval: grantcontrollers.ResyncInterval}); err != nil {
		fatal(logger, err, "add grants REST mapper reset")
	}
	jsonpathFactory := jsonpath.NewWithCache()
	// The admission webhooks read the grant model (namespaces, projects, policies, references and
	// definitions) from informers of the manager's cache registered here, on every replica, and the
	// granted objects from the granted objects cache. Neither waits for a sync inside a request: a read
	// of the manager's cache would start an informer lazily and block on its sync, which can exceed the
	// webhook deadline and pile up into a queue lock. Until an informer has synced, and while its list or
	// watch fails (grantModelWatch), the kind is read from the API server. The granted objects cache
	// lists the definitions through it too.
	grantModel, err := grantwebhooks.NewModelReader(context.Background(), runtimeManager.GetCache(), runtimeManager.GetAPIReader(), grantModelWatch)
	if err != nil {
		fatal(logger, err, "register the grant model informers of the webhooks")
	}
	// The granted objects are read from informers of a cache of their own, on every replica: the
	// catalogs of all project namespaces and the webhooks would otherwise each list the whole granted
	// kind from the API server. Until an informer has synced, the reads go to the API server.
	grantObjects, err := newGrantObjectsCache(runtimeManager, grantsMapper, grantModel)
	if err != nil {
		fatal(logger, err, "set up the granted objects cache")
	}
	grantChanges := &grantcache.Trigger{Delay: grantChangesDelay, MinInterval: grantChangesMinInterval}
	grantObjects.OnChange(grantChanges.Kick)
	// A granted kind whose CRD was deleted: the grants mapper forgets it at once instead of on its
	// next reset, and the catalogs are refreshed, so they treat the definition as not served.
	grantObjects.OnNotServed(func() {
		grantsMapper.Reset()
		grantChanges.Kick()
	})
	if err = runtimeManager.Add(grantChanges); err != nil {
		fatal(logger, err, "add the granted objects change trigger")
	}
	if err = (&grantcontrollers.ProjectReconciler{
		Client:  runtimeManager.GetClient(),
		Granted: grantObjects.Reader(runtimeManager.GetClient()),
		Memo:    &resolve.FieldsMemo{},
		Changes: grantChanges.Events(),
		Mapper:  grantsMapper,
		// Usage objects are of whatever kinds the references name; the uncached reader keeps the
		// two-minute recount from starting an informer per kind.
		Usage:    runtimeManager.GetAPIReader(),
		Factory:  jsonpathFactory,
		Recorder: runtimeManager.GetEventRecorderFor(controllerName),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up grant project reconciler")
	}
	if err = (&grantcontrollers.ReferenceReconciler{Client: runtimeManager.GetClient(), Factory: jsonpathFactory}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up grant reference reconciler")
	}
	if err = (&grantcontrollers.DefinitionReconciler{Client: runtimeManager.GetClient(), Factory: jsonpathFactory, Mapper: grantsMapper}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up grant definition reconciler")
	}
	if err = (&grantcontrollers.PolicyReconciler{Client: runtimeManager.GetClient(), Granted: grantObjects.Reader(runtimeManager.GetClient()), Mapper: grantsMapper}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up grant policy reconciler")
	}
	grantWebhookReader := grantObjects.Reader(grantModel)
	grantwebhooks.NewIsGrantedValidator(logger, grantWebhookReader, grantsMapper, jsonpathFactory).InstallInto(runtimeManager.GetWebhookServer())
	grantwebhooks.NewDefaultsMutator(logger, grantWebhookReader, grantsMapper, jsonpathFactory).InstallInto(runtimeManager.GetWebhookServer())
	grantwebhooks.NewProtectValidator(logger, serviceAccount).InstallInto(runtimeManager.GetWebhookServer())
	// Reject a GrantableClusterResourceReference whose paths /is-granted cannot compile, whose
	// defaulting path the /defaults mutator cannot write to, or whose fieldPaths scopes stray outside
	// or leave holes in spec.rule. Same factory as those two webhooks and the reference reconciler, so
	// all of them read a path the same way.
	grantableclusterresourcereferencewebhook.Register(runtimeManager, jsonpathFactory)
	// Reject a GrantableClusterResourceDefinition of a namespaced grantedResource, or whose catalogFields
	// the catalog projection would skip. Same factory and mapper as the projection and the definition
	// reconciler.
	grantableclusterresourcedefinitionwebhook.Register(runtimeManager, jsonpathFactory, grantsMapper)

	// register the project role binding reconcilers
	if err = (&projectrolebindingcontroller.Reconciler{
		Client:   runtimeManager.GetClient(),
		Recorder: runtimeManager.GetEventRecorderFor(controllerName),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up project role binding reconciler")
	}
	if err = (&clusterprojectrolebindingcontroller.Reconciler{Client: runtimeManager.GetClient()}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up cluster project role binding reconciler")
	}

	// register the project namespace reconciler
	if err = (&projectnamespacecontroller.Reconciler{
		Client:          runtimeManager.GetClient(),
		Recorder:        runtimeManager.GetEventRecorderFor(controllerName),
		SingleNamespace: singleNamespace,
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up project namespace reconciler")
	}

	// register the project quota reconciler and the webhook that checks the keys of a quota
	if err = (&projectquotacontroller.Reconciler{
		Client:    runtimeManager.GetClient(),
		Config:    quotaConfig,
		Discovery: reconcileQuotaKeys,
		Recorder:  runtimeManager.GetEventRecorderFor(controllerName),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up project quota reconciler")
	}
	projectquotawebhook.Register(runtimeManager, quotaKeys)

	// register the reconciler that reports projects named like additional namespaces of other projects
	if err = (&projectnamescontroller.Reconciler{
		Client:   runtimeManager.GetClient(),
		Recorder: runtimeManager.GetEventRecorderFor(controllerName),
	}).SetupWithManager(runtimeManager); err != nil {
		fatal(logger, err, "set up project names reconciler")
	}

	// register the project role binding webhooks
	projectrolebindingwebhook.Register(runtimeManager)
	clusterprojectrolebindingwebhook.Register(runtimeManager)
	projectnamespacewebhook.Register(runtimeManager, singleNamespace)

	// start runtime manager
	if err = runtimeManager.Start(ctx); err != nil {
		fatal(logger, err, "start runtime manager")
	}
}

// The changes of granted objects are folded into one refresh of the catalogs per grantChangesDelay,
// and the refreshes are at least grantChangesMinInterval apart: each one may rewrite the catalog of
// every project namespace.
const (
	grantChangesDelay       = 5 * time.Second
	grantChangesMinInterval = 30 * time.Second
)

// newGrantObjectsCache makes the cache of the granted objects (see grantcache) and adds it, with the
// sweep of the informers no definition needs, to the manager. The objects keep everything but their
// managedFields: catalogFields, selectors and defaultFrom may read any other part of them.
func newGrantObjectsCache(mgr ctrl.Manager, grantsMapper meta.RESTMapper, definitions client.Reader) (*grantcache.Cache, error) {
	// The definitions are read through the grant model reader of the webhooks: from the informer
	// registered on every replica once it has synced, from the API server before that, so a webhook
	// never waits for a sync.
	objects, err := grantcache.Build(mgr.GetConfig(), cache.Options{
		HTTPClient:       mgr.GetHTTPClient(),
		Scheme:           mgr.GetScheme(),
		Mapper:           mgr.GetRESTMapper(),
		DefaultTransform: cache.TransformStripManagedFields(),
	}, mgr.GetAPIReader(), definitions)
	if err != nil {
		return nil, err
	}
	if err := mgr.Add(objects); err != nil {
		return nil, err
	}
	sweeper := &grantcache.Sweeper{
		Cache:    objects,
		Mapper:   grantsMapper,
		Interval: grantcontrollers.ResyncInterval,
	}
	if err := mgr.Add(sweeper); err != nil {
		return nil, err
	}
	return objects, nil
}

// waitForCRDConversion blocks until every CRD of convertedCRDs converts through the webhook, answering
// the probes of the kubelet meanwhile. It reads the CRDs with a client of its own: the manager is
// created only after it returns.
func waitForCRDConversion(ctx context.Context, logger logr.Logger) error {
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("add the CRD types to the scheme: %w", err)
	}
	reader, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("create the client: %w", err)
	}
	listener, err := net.Listen("tcp", healthProbeAddress)
	if err != nil {
		return fmt.Errorf("listen for the probes: %w", err)
	}

	conversion := &startup.CRDConversion{
		Reader:   reader,
		Logger:   logger.WithName("crd-conversion"),
		Names:    convertedCRDs,
		Interval: crdConversionInterval,
	}
	return startup.ServeProbesWhile(ctx, listener, conversion.Wait)
}

// fatal logs the error with context and terminates the process; used for unrecoverable startup
// failures instead of panicking.
func fatal(logger logr.Logger, err error, msg string) {
	logger.Error(err, msg)
	os.Exit(1)
}

// newScheme is the scheme of the manager and of every client it hands out, the uncached API reader
// included. A typed object of a kind it lacks cannot be read at all, so every kind the controller
// reads has to be here: apps/v1 for the ReplicaSets the startup wait (startup.PreviousPods) lists.
func newScheme() (*runtime.Scheme, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		v1alpha1.AddToScheme,
		deckhousev1alpha2.AddToScheme,
		v1alpha3.AddToScheme,
		grantsv1alpha1.AddToScheme, grantsv1alpha2.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		rbacv1.AddToScheme,
		authorizationv1.AddToScheme,
		// the controller checks the webhook and the lease of the quota ledger
		admissionregistrationv1.AddToScheme,
		coordinationv1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			return nil, err
		}
	}
	return scheme, nil
}

func setupRuntimeManager(logger logr.Logger, watchError toolscache.WatchErrorHandlerWithContext) (ctrl.Manager, error) {
	scheme, err := newScheme()
	if err != nil {
		logger.Error(err, "failed to add scheme to runtime manager")
		return nil, err
	}

	opts := manager.Options{
		LeaderElection:                true,
		LeaderElectionID:              controllerName,
		LeaderElectionNamespace:       helmNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 ptr.To(leaseDuration),
		RenewDeadline:                 ptr.To(renewDeadline),
		RetryPeriod:                   ptr.To(retryPeriod),
		Scheme:                        scheme,
		GracefulShutdownTimeout:       ptr.To(10 * time.Second),
		HealthProbeBindAddress:        healthProbeAddress,
		WebhookServer:                 webhook.NewServer(webhook.Options{CertDir: "/certs"}),
		// the ProjectQuota controller watches the ConfigMaps of the ledger and its certificate; nothing
		// reads another ConfigMap or Secret through the cache
		Cache: cache.Options{DefaultWatchErrorHandler: watchError, ByObject: map[client.Object]cache.ByObject{
			&corev1.ConfigMap{}: {Namespaces: map[string]cache.Config{quota.ConfigNamespace: {}}},
			&corev1.Secret{}: {Namespaces: map[string]cache.Config{quota.ConfigNamespace: {}},
				Field: fields.OneTermEqualSelector("metadata.name", quota.LedgerCertsSecretName)},
			// the ProjectQuota controller sums the load balancers; the Services keep only what it counts by
			&corev1.Service{}: {Transform: evaluator.TrimService},
			// Every replica keeps the namespaces and the projects for the grant webhooks; nothing reads
			// their managedFields from the cache (the Helm takeover reads them as unstructured, uncached).
			&corev1.Namespace{}: {Transform: cache.TransformStripManagedFields()},
			&v1alpha3.Project{}: {Transform: cache.TransformStripManagedFields()},
		}},
		// The grant-violation series (d8_cluster_objects_grant_violated), the project-name pairs
		// (d8_multitenancy_project_named_like_additional_namespace), the handovers that have not
		// finished (d8_multitenancy_project_handover_pending) and the objects left from Helm text
		// releases (d8_multitenancy_project_kept_objects) are served from here; the PodMonitor of the
		// module scrapes this port.
		Metrics: metrics.Options{
			// Loopback only: the series name projects and the objects inside them, so the endpoint is
			// published through the kube-rbac-proxy sidecar, which authorizes every scrape.
			BindAddress: "127.0.0.1:9191",
		},
	}

	runtimeManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		logger.Error(err, "unable to create runtime manager")
		return nil, err
	}
	if err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		return nil, err
	}
	// Honest readiness: report Ready only once the webhook server is actually serving, instead of an
	// unconditional Ping. Otherwise the pod is Ready while its webhooks cannot answer yet, and the
	// apiserver routes admission to a not-yet-serving replica (webhook timeouts on startup/rollout).
	if err = runtimeManager.AddReadyzCheck("readyz", runtimeManager.GetWebhookServer().StartedChecker()); err != nil {
		logger.Error(err, "unable to set up ready check")
		return nil, err
	}
	return runtimeManager, nil
}

// logLevel parses a zap level name, falling back to info. Only the named zap levels are accepted, so
// the V(n) messages of controller-runtime below debug are dropped rather than logged as Level(-n).
func logLevel(name string) zapcore.Level {
	if name == "" {
		name = defaultLogLevel
	}
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return zapcore.InfoLevel
	}
	return level
}

// setupLogging makes one JSON logger, at the level named by LOG_LEVEL, the logger of controller-runtime
// and of klog, which client-go uses for leader election, so that every line is JSON with a named level.
func setupLogging(opts ...zap.Opts) logr.Logger {
	opts = append([]zap.Opts{zap.Level(logLevel(os.Getenv(logLevelEnv))), zap.StacktraceLevel(zapcore.PanicLevel)}, opts...)
	logger := zap.New(opts...)
	ctrl.SetLogger(logger)
	klog.SetLogger(logger)
	return logger
}

// quotaLedgerMode is the argument that runs the binary as the quota-ledger container of the pod: the
// webhook that keeps the sum of a project quota, apart from the controllers and their webhooks, so
// that it has a runtime and a memory limit of its own.
const (
	quotaLedgerMode = "quota-ledger"
	// quotaLedgerFamiliesEnv lists, comma-separated, the families of objects the ledger counts.
	quotaLedgerFamiliesEnv = "QUOTA_LEDGER_FAMILIES"
)

func runQuotaLedger() {
	logger := setupLogging().WithName(quotaLedgerMode)

	runID, err := ledger.NewRunID()
	if err != nil {
		fatal(logger, err, "make the run identifier")
	}
	opts := ledger.Options{
		Config: ctrl.GetConfigOrDie(),
		Self: ledger.Self{
			PodName: os.Getenv(podNameEnv),
			PodUID:  types.UID(os.Getenv("POD_UID")),
			PodIP:   os.Getenv("POD_IP"),
			RunID:   runID,
		},
		CertDir:        "/quota-ledger-certs",
		Logger:         logger,
		WebhookAddress: ":9444",
		HealthAddress:  ":9445",
		MetricsAddress: "127.0.0.1:9192",
	}
	// Helm lists the families of objects whose module is enabled; a change of the list rolls the pod
	if families := os.Getenv(quotaLedgerFamiliesEnv); families != "" {
		opts.Families = strings.Split(families, ",")
	}
	if err := ledger.Run(ctrl.SetupSignalHandler(), opts); err != nil {
		fatal(logger, err, "run the quota ledger")
	}
}
