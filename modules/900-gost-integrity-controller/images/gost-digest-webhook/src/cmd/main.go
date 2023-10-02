/*
Copyright 2023 Flant JSC

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
	"flag"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/deckhouse/deckhouse/modules/900-gost-integrity-controller/images/gost-digest-webhook/src/pkg/validation"
	"github.com/deckhouse/deckhouse/modules/900-gost-integrity-controller/images/gost-digest-webhook/src/pkg/webhookhandler"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	metricsAddr                 string
	probeAddr                   string
	host                        string
	port                        int
	cacheEvictionDurationSecond int
	tlsSkipVerify               bool
	defaultRegistry             string
	certDir                     string
	logLevel                    string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func parseFlags() {
	flag.StringVar(&host, "host", "127.0.0.1", "The address the webhook endpoint binds to.")
	flag.IntVar(&port, "port", 9443, "The port the webhook.")
	flag.StringVar(
		&metricsAddr,
		"metrics-bind-address",
		":8080",
		"The address the metric endpoint binds to.",
	)
	flag.StringVar(
		&probeAddr,
		"health-probe-bind-address",
		":8081",
		"The address the probe endpoint binds to.",
	)
	flag.IntVar(
		&cacheEvictionDurationSecond,
		"cache-eviction",
		60,
		"Cache eviction duration in seconds",
	)
	flag.BoolVar(
		&tlsSkipVerify,
		"insecure",
		false,
		"Allow image references to be fetched without TLS",
	)
	flag.StringVar(
		&defaultRegistry,
		"default-registry",
		name.DefaultRegistry,
		"Default registry",
	)
	flag.StringVar(
		&certDir,
		"cert-dir",
		"./cert",
		"Directory with certificates",
	)
	flag.StringVar(
		&logLevel,
		"log-level",
		"info",
		"Webhook logging level (debug, info, error)",
	)
}

func main() {
	parseFlags()

	zapLogLevel, err := zapcore.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		os.Exit(1)
	}

	opts := zap.Options{
		Development: true,
		Level:       zapLogLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(
			webhook.Options{
				CertDir: certDir,
				Host:    host,
				Port:    port,
			},
		),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		setupLog.Error(err, "can't init kubernetes client in cluster")
		os.Exit(1)
	}

	mgr.GetWebhookServer().Register(
		"/validate",
		&webhook.Admission{
			Handler: webhookhandler.NewHandler(
				validation.NewGostDigestValidation(
					tlsSkipVerify,
					defaultRegistry,
					cacheEvictionDurationSecond,
					kubeClient,
				),
			),
			RecoverPanic: true,
		},
	)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager was stopped with error")
		os.Exit(1)
	}
}

func newKubeClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
