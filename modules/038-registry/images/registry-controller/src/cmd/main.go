/*
Copyright 2026 Flant JSC

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

	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/register"
	_ "github.com/deckhouse/registry-controller/internal/register/controllers"
)

var (
	scheme     = runtime.NewScheme()
	logOptions = logs.NewOptions()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(registryv1alpha1.AddToScheme(scheme))
}

// registryNamespace is where this module's objects live. The same namespace the reconciler
// reads its secrets from, named once so the cache and the reads cannot disagree about it.
const registryNamespace = "d8-system"

func main() {
	var (
		metricsAddr             string
		probeAddr               string
		disabledControllers     string
		maxConcurrentReconciles int
		leaderElect             bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "127.0.0.1:4281",
		"The address the metrics endpoint binds to. Loopback, because what the cluster "+
			"scrapes is the kube-rbac-proxy beside it.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":4282", "The address the probe endpoint binds to.")
	flag.StringVar(&logOptions.Format, "logging-format", logOptions.Format, "Logging format (text or json).")
	flag.StringVar(&disabledControllers, "disable-controllers", "",
		"Comma-separated list of controllers to disable. For incident response: a misbehaving reconciler "+
			"can be switched off without rolling back the image.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 5,
		"Maximum number of concurrent reconciles per controller.")
	flag.BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for the controller manager.")

	logs.AddGoFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(klog.Background())
	setupLog := ctrl.Log.WithName("setup")

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to validate and apply log options")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,

		// Secrets are cached for one namespace, not for the cluster.
		//
		// Not an optimization. A cached client backs every read with an informer, and an
		// informer lists what it watches — so a cluster-wide cache would need cluster-wide
		// list on secrets, which is precisely the permission this module went to some
		// trouble to avoid needing. Confined here, the namespaced grants by resource name
		// are enough. Without this the informer's first list is refused, the cache never
		// starts, and every controller in this manager stalls with nothing to say beyond a
		// forbidden error on a resource it only ever wanted two objects from.
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{registryNamespace: {}},
				},
			},
		},

		LeaderElection:          leaderElect,
		LeaderElectionID:        "registry-controller.deckhouse.io",
		LeaderElectionNamespace: os.Getenv("POD_NAMESPACE"),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := register.SetupAll(mgr, mgr.GetClient(), splitList(disabledControllers), maxConcurrentReconciles); err != nil {
		setupLog.Error(err, "unable to set up controllers")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up the health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up the readiness check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func splitList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
