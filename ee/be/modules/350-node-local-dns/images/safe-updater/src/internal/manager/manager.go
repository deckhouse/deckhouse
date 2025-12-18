/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package manager

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"safe-updater/internal/constant"
	controller "safe-updater/internal/controller"
)

type Manager struct {
	runtimeManager manager.Manager
}

func (c *Manager) Start(ctx context.Context) error {
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			klog.Fatalf("start manager failed: %v", err)
		}
	}()
	klog.Info("manager started")

	if ok := c.runtimeManager.GetCache().WaitForCacheSync(ctx); !ok {
		return fmt.Errorf("wait for cache sync")
	}
	klog.Info("cache synced")

	return nil
}

func NewManager(ctx context.Context, pprofEnabled bool) (*Manager, error) {
	kubeClient, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}

	addToScheme := []func(s *runtime.Scheme) error{
		appsv1.AddToScheme,
		corev1.AddToScheme,
	}

	scheme := runtime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			return nil, fmt.Errorf("add to scheme: %w", err)
		}
	}

	controllerruntime.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	pprofAddr := ""
	if pprofEnabled {
		pprofAddr = constant.PprofBindAddress
	}

	runtimeManager, err := controllerruntime.NewManager(kubeClient, controllerruntime.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: constant.ControllerName,
		BaseContext: func() context.Context {
			return ctx
		},
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress:  constant.HealthProbeBindAddress,
		PprofBindAddress:        pprofAddr,
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
		Cache: cache.Options{
			ReaderFailOnMissingInformer: false,
			DefaultTransform:            cache.TransformStripManagedFields(),
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						constant.NodeLocalDNSNamespace: {
							LabelSelector: constant.NodeLocalDNSPodLabelSelector,
						},
						constant.CiliumNamespace: {
							LabelSelector: constant.CiliumAgentPodLabelSelector,
						},
					},
				},
				&appsv1.DaemonSet{}: {
					Namespaces: map[string]cache.Config{
						constant.NodeLocalDNSNamespace: {
							LabelSelector: constant.NodeLocalDNSDSLabelSelector,
						},
						constant.CiliumNamespace: {
							LabelSelector: constant.CiliumAgentPodLabelSelector,
						},
					},
				},
				&appsv1.ControllerRevision{}: {
					Namespaces: map[string]cache.Config{
						constant.NodeLocalDNSNamespace: {
							LabelSelector: constant.ControllerRevisionLabelSelector,
						},
						constant.CiliumNamespace: {
							LabelSelector: constant.CiliumAgentPodLabelSelector,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create controller runtime manager: %w", err)
	}

	if err = runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add health check: %w", err)
	}
	if err = runtimeManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add ready check: %w", err)
	}

	if err = controller.RegisterController(runtimeManager); err != nil {
		return nil, fmt.Errorf("add controller: %w", err)
	}

	return &Manager{
		runtimeManager: runtimeManager,
	}, nil
}
