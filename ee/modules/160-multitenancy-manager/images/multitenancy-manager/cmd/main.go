/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	projectcontroller "controller/pkg/controller/project"
	templatecontroller "controller/pkg/controller/template"
	"controller/pkg/helm"
	namespacewebhook "controller/pkg/webhook/namespace"
	projectwebhook "controller/pkg/webhook/project"
	templatewebhook "controller/pkg/webhook/template"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	// path to helm templates
	templatesPath = "templates"
	// path to default project templates
	defaultPath = "default"
	// helm release namespace
	helmNamespace = "d8-multitenancy-manager"
	// controller service account
	serviceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	// deckhouse service account
	deckhouseServiceAccount = "system:serviceaccount:d8-system:deckhouse"
)

func main() {
	var allowOrphanNamespaces bool
	flag.BoolVar(&allowOrphanNamespaces, "allow-orphan-namespaces", true, "allow to create a namespace which is not a part of a Project")
	flag.Parse()

	// setup logger
	log := ctrl.Log.WithName("multitenancy-manager")
	ctrllog.SetLogger(zap.New(zap.Level(zapcore.Level(-4)), zap.UseDevMode(true)))

	log.Info(fmt.Sprintf("starting multitenancy-manager with %v allow orphan namespaces option", allowOrphanNamespaces))

	// initialize runtime manager
	runtimeManager, err := setupRuntimeManager(log)
	if err != nil {
		panic(err)
	}

	// initialize helm client
	helmClient, err := helm.New(helmNamespace, templatesPath, log)
	if err != nil {
		panic(err)
	}

	// register project controller
	if err = projectcontroller.Register(runtimeManager, helmClient, log); err != nil {
		panic(err)
	}

	// register template controller
	if err = templatecontroller.Register(runtimeManager, defaultPath, log); err != nil {
		panic(err)
	}

	// register project webhook
	projectwebhook.Register(runtimeManager)

	// register template webhook
	templatewebhook.Register(runtimeManager, serviceAccount)

	if !allowOrphanNamespaces {
		// register namespace webhook
		namespacewebhook.Register(runtimeManager, serviceAccount, deckhouseServiceAccount)
	}

	// start runtime manager
	if err = runtimeManager.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func setupRuntimeManager(log logr.Logger) (ctrl.Manager, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		v1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			log.Error(err, "failed to add scheme to runtime manager")
			return nil, err
		}
	}

	managerOpts := manager.Options{
		LeaderElection:          false,
		Scheme:                  scheme,
		GracefulShutdownTimeout: pointer.Duration(10 * time.Second),
		HealthProbeBindAddress:  ":9090",
		WebhookServer:           webhook.NewServer(webhook.Options{CertDir: "/certs"}),
		Metrics: metrics.Options{
			BindAddress: "0",
		},
	}

	runtimeManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpts)
	if err != nil {
		log.Error(err, "unable to create runtime manager")
		return nil, err
	}
	if err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = runtimeManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	return runtimeManager, nil
}
