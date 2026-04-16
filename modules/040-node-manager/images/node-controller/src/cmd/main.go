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
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
	_ "github.com/deckhouse/node-controller/internal/register/controllers"
	"github.com/deckhouse/node-controller/internal/webhook"
)

var (
	scheme     = runtime.NewScheme()
	logOptions = logs.NewOptions()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deckhousev1.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha1.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var disabledControllers string
	var maxConcurrentReconcilesRaw string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&logOptions.Format, "logging-format", logOptions.Format, "Logging format (text or json)")
	flag.StringVar(&disabledControllers, "disable-controllers", "", "Comma-separated list of controllers to disable")
	flag.StringVar(&maxConcurrentReconcilesRaw, "max-concurrent-reconciles", "10", "Maximum number of concurrent reconciles per controller. Format: N or N,controller1=M,controller2=K")

	logs.AddGoFlags(flag.CommandLine)

	flag.Parse()
	ctrl.SetLogger(klog.Background())
	setupLog := ctrl.Log.WithName("setup")

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to validate and apply log options")
		os.Exit(1)
	}

	cfg := ctrl.GetConfigOrDie()
	ctx := ctrl.SetupSignalHandler()

	// Webhook manager — separate cache, no informers for Node/Endpoints.
	webhookCacheOpts, webhookClientOpts := common.WebhookCacheOptions()
	webhookMgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port: 9443,
		}),
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Cache:                  webhookCacheOpts,
		Client:                 webhookClientOpts,
	})
	if err != nil {
		setupLog.Error(err, "unable to start webhook manager")
		os.Exit(1)
	}

	if err = webhook.SetupWithManager(webhookMgr); err != nil {
		setupLog.Error(err, "unable to setup webhooks")
		os.Exit(1)
	}

	go func() {
		setupLog.Info("starting webhook manager")
		if err := webhookMgr.Start(ctx); err != nil {
			setupLog.Error(err, "webhook manager failed")
			os.Exit(1)
		}
	}()

	// Controller manager — full cache for controllers, no webhook server.
	// Secrets are cached only from d8-cloud-instance-manager and kube-system namespaces.
	ctrlMgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		Cache:                  common.ControllerCacheOptions(ctx, setupLog),
	})
	if err != nil {
		setupLog.Error(err, "unable to start controller manager")
		os.Exit(1)
	}

	defaultMaxConcurrent, perControllerMaxConcurrent, err := parseMaxConcurrentReconciles(maxConcurrentReconcilesRaw)
	if err != nil {
		setupLog.Error(err, "invalid --max-concurrent-reconciles value, falling back to defaults")
		defaultMaxConcurrent = defaultMaxConcurrentReconciles
		perControllerMaxConcurrent = nil
	}
	setupLog.V(1).Info("max-concurrent-reconciles parsed", "default", defaultMaxConcurrent, "perController", perControllerMaxConcurrent)

	if err = register.SetupAll(ctrlMgr, disabledControllers, defaultMaxConcurrent, perControllerMaxConcurrent); err != nil {
		setupLog.Error(err, "unable to setup controllers")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := ctrlMgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := ctrlMgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Log cache contents after sync.
	go func() {
		if ctrlMgr.GetCache().WaitForCacheSync(ctx) {
			common.LogCacheContents(ctx, ctrlMgr.GetCache(), setupLog)
		}
	}()

	setupLog.Info("starting controller manager")
	if err := ctrlMgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running controller manager")
		os.Exit(1)
	}
}

const defaultMaxConcurrentReconciles = 10

// parseMaxConcurrentReconciles parses the --max-concurrent-reconciles flag value.
// Supported formats:
//   - "N" — global default for all controllers (e.g. "10")
//   - "controller1=N,controller2=M" — per-controller values, global default is 10
//   - "N,controller1=M" — global default N with per-controller overrides
func parseMaxConcurrentReconciles(raw string) (int, map[string]int, error) {
	globalDefault := defaultMaxConcurrentReconciles
	perController := make(map[string]int)

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if k, v, ok := strings.Cut(part, "="); ok {
			name := strings.TrimSpace(k)
			valStr := strings.TrimSpace(v)
			val, err := strconv.Atoi(valStr)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid value for controller %q: %q", name, valStr)
			}
			if val < 1 {
				return 0, nil, fmt.Errorf("max-concurrent-reconciles for controller %q must be >= 1, got %d", name, val)
			}
			perController[name] = val
		} else {
			val, err := strconv.Atoi(part)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid max-concurrent-reconciles value: %q", part)
			}
			if val < 1 {
				return 0, nil, fmt.Errorf("max-concurrent-reconciles must be >= 1, got %d", val)
			}
			globalDefault = val
		}
	}

	return globalDefault, perController, nil
}
