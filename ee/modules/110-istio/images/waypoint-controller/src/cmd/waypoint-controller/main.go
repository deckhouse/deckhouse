/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"waypoint-controller/internal/waypointcontroller"
	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

var (
	healthProbeBindAddress string
	leaderElect            bool
)

func newManager(healthProbeBindAddress string, leaderElect bool, vpaEnabled bool) (manager.Manager, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientsetscheme.AddToScheme(scheme))
	utilruntime.Must(policyv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv2.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(networkv1alpha1.AddToScheme(scheme))
	if vpaEnabled {
		utilruntime.Must(vpav1.AddToScheme(scheme))
	}

	controllerNs := os.Getenv("POD_NAMESPACE")
	if controllerNs == "" {
		controllerNs = "d8-istio"
	}

	managedResourcesCache := cache.ByObject{
		Namespaces: map[string]cache.Config{
			cache.AllNamespaces: {},
		},
		Label: labels.SelectorFromSet(map[string]string{
			waypointcontroller.AppLabelKey: waypointcontroller.AppLabelValue,
		}),
	}

	byObject := map[client.Object]cache.ByObject{
		&networkv1alpha1.WaypointInstance{}: {
			Namespaces: map[string]cache.Config{
				cache.AllNamespaces: {},
			},
		},
		&appsv1.Deployment{}:                     managedResourcesCache,
		&corev1.Service{}:                        managedResourcesCache,
		&corev1.ServiceAccount{}:                 managedResourcesCache,
		&policyv1.PodDisruptionBudget{}:          managedResourcesCache,
		&autoscalingv2.HorizontalPodAutoscaler{}: managedResourcesCache,
		&gatewayv1.Gateway{}:                     managedResourcesCache,
	}
	if vpaEnabled {
		byObject[&vpav1.VerticalPodAutoscaler{}] = managedResourcesCache
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
		Cache: cache.Options{
			ByObject: byObject,
		},
		LeaderElection:          leaderElect,
		LeaderElectionID:        "d8-waypoint",
		LeaderElectionNamespace: controllerNs,
		HealthProbeBindAddress:  healthProbeBindAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("create manager: %w", err)
	}

	return mgr, nil
}

func vpaEnabledFromEnv() (bool, error) {
	value := os.Getenv("VPA_ENABLED")
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse VPA_ENABLED: %w", err)
	}

	return parsed, nil
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	// controller-runtime requires a logr.Logger. Prefer textlogger over klogr.New()
	// because klogr.New() is deprecated in klog/v2.
	logCfg := textlogger.NewConfig(
		textlogger.VerbosityFlagName("controller-log-level"),
		textlogger.VModuleFlagName("controller-vmodule"),
	)
	logCfg.AddFlags(flag.CommandLine)
	ctrl.SetLogger(textlogger.NewLogger(logCfg))

	flag.StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":9239", "Sets the address for the health check endpoint.")
	flag.BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctx := signals.SetupSignalHandler()

	vpaEnabled, err := vpaEnabledFromEnv()
	if err != nil {
		klog.Error(err, "Failed to parse VPA_ENABLED env variable")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err := waypointcontroller.WaitForGatewayAPICRDCompliance(ctx); err != nil {
		klog.Error(err, "Failed to ensure Gateway API CRD compliance")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	mgr, err := newManager(healthProbeBindAddress, leaderElect, vpaEnabled)
	if err != nil {
		klog.Error(err, "Failed to create manager")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err := (&waypointcontroller.WaypointController{VPAEnabled: vpaEnabled}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "Failed to set up waypoint controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Error(err, "Failed to set up healthz endpoint")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Error(err, "Failed to set up readyz endpoint")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err := mgr.Start(ctx); err != nil {
		klog.Error(err, "Failed to run manager")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
