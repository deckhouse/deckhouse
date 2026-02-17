/*
Copyright 2025 Flant JSC

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

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"

	"github.com/deckhouse/node-controller/internal/controller"
	"github.com/deckhouse/node-controller/internal/webhook"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme     = runtime.NewScheme()
	logOptions = logs.NewOptions()
)

func init() {
	// core
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// deckhosue
	utilruntime.Must(deckhousev1.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha1.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha2.AddToScheme(scheme))
	// capi
	utilruntime.Must(capiv1beta2.AddToScheme(scheme))
	// mcm
	utilruntime.Must(mcmv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&logOptions.Format, "logging-format", logOptions.Format, "Logging format (text or json)")

	logs.AddGoFlags(flag.CommandLine)

	flag.Parse()
	ctrl.SetLogger(klog.Background())
	setupLog := ctrl.Log.WithName("setup")

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to validate and apply log options")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				controller.MachineNamespace: {},
				"kube-system":               {},
				"default":                   {},
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = webhook.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "NodeGroup")
		os.Exit(1)
	}

	if err = controller.SetupInstanceController(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Instance")
		os.Exit(1)
	}

	if err = controller.SetupMCMMachineController(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MCMMachine")
		os.Exit(1)
	}

	if err = controller.SetupCAPIMachineController(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CAPIMachine")
		os.Exit(1)
	}

	if err = controller.SetupNodeController(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
