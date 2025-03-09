/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"service-with-healthchecks/internal/agent"
	"syscall"

	"github.com/go-logr/logr"
	_ "go.uber.org/automaxprocs" // To automatically adjust GOMAXPROCS

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var pprofAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var debugging bool
	var workersCount int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":9874", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9873", "The controller healthz bind address.")
	flag.StringVar(&pprofAddr, "pprof-bind-address", "", "The address the pprof binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&debugging, "debugging", false, "If set, enables debugging")
	flag.IntVar(&workersCount, "workers-count", 4, "The number of workers to run")

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		setupLog.Error(fmt.Errorf("Eviroment variable NODE_NAME is not defined"), "unable to start controller")
		os.Exit(1)
	}

	opts := zap.Options{
		Development: debugging,
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

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		PprofBindAddress:       pprofAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	secretController := &agent.PostgreSQLCredentialsReconciler{
		Client: mgr.GetClient(),
		Logger: ctrl.Log.WithName("secret-controller"),
		Scheme: mgr.GetScheme(),
	}
	if err = secretController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PostgreSQLCredentials")
		os.Exit(1)
	}

	serviceWithHealthchecksController := agent.NewServiceWithHealthchecksReconciler(
		mgr.GetClient(),
		workersCount,
		nodeName,
		mgr.GetScheme(),
		ctrl.Log.WithName("service-with-healthchecks-controller"),
		secretController,
	)
	if err = serviceWithHealthchecksController.RunWorkers(context.Background()); err != nil {
		setupLog.Error(err, "unable to run controller workers", "controller", "ServiceWithHealthchecks")
		os.Exit(1)
	}

	if err = serviceWithHealthchecksController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceWithHealthchecks")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(SetupSignalHandler(serviceWithHealthchecksController, setupLog)); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func SetupSignalHandler(reconciler *agent.ServiceWithHealthchecksReconciler, logger logr.Logger) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("received signal, shutting down reconciler")
		cancel()
		reconciler.Shutdown()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
