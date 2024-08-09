/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"flag"
	"os"
	"time"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	projectcontroller "controller/pkg/controller/project"
	templatecontroller "controller/pkg/controller/template"
	"controller/pkg/helm"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	templatesPath = "templates"
	defaultPath   = "default"
	helmNamespace = "d8-multitenancy-manager"
	log           = ctrl.Log.WithName("multitenancy-manager")
)

func main() {
	var probeAddr string
	flag.StringVar(&probeAddr, "health-probe-address", ":0", "The address the probe endpoint binds to.")
	flag.Parse()

	// setup logger
	ctrllog.SetLogger(zap.New(zap.Level(zapcore.Level(-4)), zap.UseDevMode(true)))

	// initialize runtime manager
	runtimeManager, err := setupRuntimeManager(probeAddr)
	if err != nil {
		panic(err)
	}

	// initialize helm client
	helmClient, err := helm.New(helmNamespace, templatesPath, log)
	if err != nil {
		panic(err)
	}

	// register project controller
	if err = projectcontroller.Register(context.Background(), runtimeManager, helmClient, log, defaultPath); err != nil {
		panic(err)
	}

	// register template controller
	if err = templatecontroller.Register(runtimeManager, log); err != nil {
		panic(err)
	}

	// start runtime manager
	if err = runtimeManager.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func setupRuntimeManager(probeAddress string) (ctrl.Manager, error) {
	addToScheme := []func(s *runtime.Scheme) error{
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
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
		HealthProbeBindAddress:  probeAddress,
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
