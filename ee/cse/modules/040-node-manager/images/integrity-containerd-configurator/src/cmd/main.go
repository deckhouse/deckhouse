/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

//nolint:goimports,gci
import (
	"flag"
	"os"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/deckhouse/deckhouse/pkg/log"

	"integrity-containerd-configurator/internal/configapplier"
	"integrity-containerd-configurator/internal/controller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Default().With("logger", "setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha1.AddToScheme(scheme))
}

func main() {
	var probeAddr string
	var configDir string
	var debugging bool

	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configDir, "config-dir", configapplier.IntegrityNSConfigDir, "Directory for containerd integrity config files.")
	flag.BoolVar(&debugging, "debug", false, "If set, enables debug logging.")

	flag.Parse()

	if debugging {
		log.SetDefaultLevel(log.LevelDebug)
	}

	ctrl.SetLogger(logr.FromSlogHandler(log.Default().Handler()))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		setupLog.Error("unable to start manager", log.Err(err))
		os.Exit(1)
	}

	if err = (&controller.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		FSApplier: configapplier.NewFSApplier(configDir),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error("unable to create controller", log.Err(err), "controller", "ContainerdIntegrityConfigurator")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error("unable to set up health check", log.Err(err))
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error("unable to set up ready check", log.Err(err))
		os.Exit(1)
	}

	setupLog.Info("starting manager", "config_dir", configDir)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error("problem running manager", log.Err(err))
		os.Exit(1)
	}
}
