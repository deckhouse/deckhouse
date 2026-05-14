/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"os"
	"regexp"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/egress-gateway-agent/internal/controller"
	"github.com/deckhouse/deckhouse/egress-gateway-agent/internal/layer2"
	internalv1alpha1 "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/internal.network/v1alpha1"
	networkv1alpha1 "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/v1alpha1"
)

const (
	excludeInterfacesPrefixes = "^(cilium_|lxc)"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Default().With("logger", "setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkv1alpha1.AddToScheme(scheme))
	utilruntime.Must(internalv1alpha1.AddToScheme(scheme))
}

func main() {
	var probeAddr string
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9870", "The address the probe endpoint binds to.")
	flag.Parse()

	// Set Deckhouse standard logger for controller-runtime
	ctrl.SetLogger(logr.FromSlogHandler(log.Default().Handler()))

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

	tlsOpts := make([]func(*tls.Config), 0, 1)
	tlsOpts = append(tlsOpts, disableHTTP2)

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
		Metrics:                metricsserver.Options{BindAddress: ":0"},
	})
	if err != nil {
		setupLog.Error("unable to start manager", "error", err)
		os.Exit(1)
	}

	nodeName := os.Getenv("NODE_NAME") // we need node name for label filters
	if nodeName == "" {
		setupLog.Error("environment variable NODE_NAME not set", "error", errors.New("NODE_NAME not set"))
		os.Exit(1)
	}

	announceLogger := EmptyLogger{}
	excludeRegex := regexp.MustCompile(excludeInterfacesPrefixes)
	virtualIPAnnounces, err := layer2.New(announceLogger, excludeRegex)
	if err != nil {
		setupLog.Error("unable to create virtual IP announcement", "error", err)
		os.Exit(1)
	}

	if err = (&controller.EgressGatewayInstanceReconciler{
		NodeName:           nodeName,
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		VirtualIPAnnounces: virtualIPAnnounces,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error("unable to create controller", "controller", "EgressGateway", "error", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error("problem running manager", "error", err)
		os.Exit(1)
	}
}

type EmptyLogger struct{}

func (e EmptyLogger) Log(keyvals ...interface{}) error {
	return nil
}
