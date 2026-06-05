/*
Copyright 2026.

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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	sh_pkg "github.com/flant/shell-operator/pkg"
	sh_app "github.com/flant/shell-operator/pkg/app"
	hook_types "github.com/flant/shell-operator/pkg/hook/types"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/deckhouse/deckhouse/pkg/log"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"deckhouse.io/webhook/internal/controller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(deckhouseiov1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// reloadHooks re-discovers hooks from disk so that shell-operator picks up
// new or removed webhook configurations without a process restart.
//
// HookManager.Init() replaces the hook index atomically, but newly loaded
// hooks have uninitialised AdmissionLinks/ConversionLinks maps. We must
// call EnableAdmissionBindings / EnableConversionBindings on every hook that
// carries validating/mutating/conversion configs so that
// CanHandleAdmissionEvent / CanHandleConversionEvent can match incoming
// requests to the right hook.
//
// AdmissionWebhookManager.Init() and ConversionWebhookManager.Init() must
// NOT be called here because they recreate HTTP servers and would either
// fail with "address already in use" or silently orphan the old listeners.
func reloadHooks(ctx context.Context, shOp *shell_operator.ShellOperator, logger *log.Logger) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before reload: %w", err)
	}

	logger.Info("reloading shell-operator hooks")

	if shOp.HookManager == nil {
		return fmt.Errorf("hook manager is not initialized")
	}

	// Snapshot the current registrations before any mutation.  These are
	// compared against the rebuilt state later to determine which
	// ValidatingWebhookConfigurations / MutatingWebhookConfigurations / CRD
	// conversion settings are stale and must be unregistered.
	oldValidatingResources := make(map[string]*admission.ValidatingWebhookResource)
	oldMutatingResources := make(map[string]*admission.MutatingWebhookResource)
	oldConversionClientConfigs := make(map[string]*conversion.CrdClientConfig)

	if shOp.AdmissionWebhookManager != nil {
		for confID, resource := range shOp.AdmissionWebhookManager.ValidatingResources {
			oldValidatingResources[confID] = resource
		}
		for confID, resource := range shOp.AdmissionWebhookManager.MutatingResources {
			oldMutatingResources[confID] = resource
		}
	}

	if shOp.ConversionWebhookManager != nil {
		for crdName, cfg := range shOp.ConversionWebhookManager.ClientConfigs {
			oldConversionClientConfigs[crdName] = cfg
		}
	}

	// Re-discover hooks from disk.  This is the step most likely to fail
	// (e.g. malformed hook file), so it MUST happen before we clear the
	// current registration maps — otherwise a failed Init leaves the
	// shell-operator with empty maps and no way to unregister stale entries.
	if err := shOp.HookManager.Init(); err != nil {
		return fmt.Errorf("re-init hook manager: %w", err)
	}

	// Clear the current registration state AFTER Init has succeeded.
	// Rebuilding from scratch avoids stale webhook resources after hook
	// removals.  The Enable*Bindings calls below will repopulate these maps
	// with fresh entries for every discovered hook.
	if shOp.AdmissionWebhookManager != nil {
		shOp.AdmissionWebhookManager.ValidatingResources = make(map[string]*admission.ValidatingWebhookResource)
		shOp.AdmissionWebhookManager.MutatingResources = make(map[string]*admission.MutatingWebhookResource)
	}

	if shOp.ConversionWebhookManager != nil {
		shOp.ConversionWebhookManager.ClientConfigs = make(map[string]*conversion.CrdClientConfig)
	}

	// Enable admission bindings on every newly loaded hook so that
	// AdmissionLinks are populated and CanHandleEvent works.
	admissionHookNames, err := shOp.HookManager.GetHooksInOrder(hook_types.KubernetesValidating)
	if err != nil {
		return fmt.Errorf("get validating hooks: %w", err)
	}
	mutatingHookNames, err := shOp.HookManager.GetHooksInOrder(hook_types.KubernetesMutating)
	if err != nil {
		return fmt.Errorf("get mutating hooks: %w", err)
	}
	for _, name := range append(admissionHookNames, mutatingHookNames...) {
		h := shOp.HookManager.GetHook(name)
		if h != nil {
			h.HookController.EnableAdmissionBindings()
		}
	}

	// Enable conversion bindings on every newly loaded hook so that
	// CanHandleConversionEvent works.
	conversionHookNames, err := shOp.HookManager.GetHooksInOrder(hook_types.KubernetesConversion)
	if err != nil {
		return fmt.Errorf("get conversion hooks: %w", err)
	}
	for _, name := range conversionHookNames {
		h := shOp.HookManager.GetHook(name)
		if h != nil {
			h.HookController.EnableConversionBindings()
		}
	}

	if err := syncAdmissionWebhookConfigurations(ctx, shOp, oldValidatingResources, oldMutatingResources); err != nil {
		return fmt.Errorf("sync admission webhook configurations: %w", err)
	}

	if err := syncConversionWebhookConfigurations(ctx, shOp, oldConversionClientConfigs); err != nil {
		return fmt.Errorf("sync conversion webhook configurations: %w", err)
	}

	return nil
}

func syncAdmissionWebhookConfigurations(
	ctx context.Context,
	shOp *shell_operator.ShellOperator,
	oldValidatingResources map[string]*admission.ValidatingWebhookResource,
	oldMutatingResources map[string]*admission.MutatingWebhookResource,
) error {
	if shOp.AdmissionWebhookManager == nil {
		return nil
	}

	for confID, resource := range shOp.AdmissionWebhookManager.ValidatingResources {
		if err := resource.Register(ctx); err != nil {
			return fmt.Errorf("register validating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range shOp.AdmissionWebhookManager.MutatingResources {
		if err := resource.Register(ctx); err != nil {
			return fmt.Errorf("register mutating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range oldValidatingResources {
		if _, stillPresent := shOp.AdmissionWebhookManager.ValidatingResources[confID]; stillPresent {
			continue
		}

		if err := resource.Unregister(); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale validating webhook configuration %q: %w", confID, err)
		}
	}

	for confID, resource := range oldMutatingResources {
		if _, stillPresent := shOp.AdmissionWebhookManager.MutatingResources[confID]; stillPresent {
			continue
		}

		if err := resource.Unregister(); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale mutating webhook configuration %q: %w", confID, err)
		}
	}

	return nil
}

func syncConversionWebhookConfigurations(
	ctx context.Context,
	shOp *shell_operator.ShellOperator,
	oldConversionClientConfigs map[string]*conversion.CrdClientConfig,
) error {
	if shOp.ConversionWebhookManager == nil {
		return nil
	}

	for crdName, cfg := range shOp.ConversionWebhookManager.ClientConfigs {
		if err := cfg.Update(ctx); err != nil {
			return fmt.Errorf("update conversion client config for crd %q: %w", crdName, err)
		}
	}

	for crdName := range oldConversionClientConfigs {
		if _, stillPresent := shOp.ConversionWebhookManager.ClientConfigs[crdName]; stillPresent {
			continue
		}

		if err := resetCRDConversionToNone(ctx, shOp, crdName); err != nil {
			return fmt.Errorf("cleanup stale conversion webhook config for crd %q: %w", crdName, err)
		}
	}

	return nil
}

func resetCRDConversionToNone(ctx context.Context, shOp *shell_operator.ShellOperator, crdName string) error {
	if shOp.KubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	crd, err := shOp.KubeClient.ApiExt().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get CRD %q: %w", crdName, err)
	}

	if crd.Spec.Conversion == nil || crd.Spec.Conversion.Strategy != apiextensionsv1.WebhookConverter {
		return nil
	}

	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.NoneConverter,
	}

	if _, err := shOp.KubeClient.ApiExt().CustomResourceDefinitions().Update(ctx, crd, sh_pkg.DefaultUpdateOptions()); err != nil {
		return fmt.Errorf("update CRD %q conversion strategy: %w", crdName, err)
	}

	return nil
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	// healthcheck value
	var isAlive = true
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e630767c.deckhouse.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	validationTpl, err := os.ReadFile("internal/controller/templates/validationwebhook.tpl")
	if err != nil {
		setupLog.Error(err, "unable to read template file")
		os.Exit(1)
	}
	conversionTpl, err := os.ReadFile("internal/controller/templates/conversionwebhook.tpl")
	if err != nil {
		setupLog.Error(err, "unable to read template file")
		os.Exit(1)
	}

	// hooks/ must exist before shell-operator discovers hooks.
	if err := os.MkdirAll("hooks", 0755); err != nil {
		setupLog.Error(err, "create hooks directory")
		os.Exit(1)
	}

	logger := log.NewLogger(
		log.WithLevel(log.LogLevelFromStr(os.Getenv("LOG_LEVEL")).Level()),
		log.WithHandlerType(log.TextHandlerType))

	// One signal context for the whole process: cancelling it stops the manager
	// and tells shell-operator to drain.
	ctx := ctrl.SetupSignalHandler()

	// --- Initialise shell-operator as an in-process library ---
	shCfg := sh_app.NewConfig()
	if err := sh_app.ParseEnv(shCfg); err != nil {
		setupLog.Error(err, "unable to parse shell-operator config from env")
		os.Exit(1)
	}

	shOp, err := shell_operator.NewShellOperator(ctx, shCfg, shell_operator.WithLogger(logger.Named("shell-operator")))
	if err != nil {
		setupLog.Error(err, "unable to initialise shell-operator")
		os.Exit(1)
	}

	// reloadFn is the callback that reconcilers invoke when hooks change on
	// disk.  It re-discovers hooks and re-registers webhook configurations
	// without restarting the shell-operator process.
	//
	// A mutex serialises concurrent calls from the validation and conversion
	// reconcilers.  Without it, overlapping HookManager.Init() +
	// Enable*Bindings sequences can operate on stale/overwritten hook indices.
	var reloadMu sync.Mutex
	reloadFn := func(ctx context.Context) error {
		reloadMu.Lock()
		defer reloadMu.Unlock()

		return reloadHooks(ctx, shOp, logger.Named("shell-operator"))
	}

	validationReconciler := controller.NewValidationWebhookReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		logger,
		string(validationTpl),
		reloadFn,
	)
	if err := validationReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup controller", "controller", "ValidationWebhook")
		os.Exit(1)
	}

	conversionReconciler := controller.NewConversionWebhookReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		logger,
		string(conversionTpl),
		reloadFn,
	)
	if err := conversionReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup controller", "controller", "ConversionWebhook")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", func(req *http.Request) error {
		if !isAlive {
			return fmt.Errorf("something went wrong")
		}
		return nil
	}); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	// The readyz check succeeds once the manager is running.  Shell-operator
	// is started before the manager, so by the time readyz is reachable the
	// webhook servers are already listening.
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start shell-operator.  The method is non-blocking (it launches internal
	// goroutines and returns) and idempotent.
	if err := shOp.Start(ctx); err != nil {
		setupLog.Error(err, "unable to start shell-operator")
		os.Exit(1)
	}

	// Start the controller-runtime manager (blocks until ctx is cancelled).
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	// Gracefully shut down shell-operator.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := shOp.Shutdown(shutdownCtx); err != nil {
		logger.Error("shell-operator shutdown failed", log.Err(err))
	}
}
