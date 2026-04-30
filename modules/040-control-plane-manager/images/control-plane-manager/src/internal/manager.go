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

package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	updateobserverv1 "control-plane-manager/internal/controllers/update-observer/pkg/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"control-plane-manager/internal/constants"
	controlplaneconfiguration "control-plane-manager/internal/controllers/control-plane-configuration"
	controlplanenode "control-plane-manager/internal/controllers/control-plane-node"
	controlplaneoperation "control-plane-manager/internal/controllers/control-plane-operation"
	operationsapprover "control-plane-manager/internal/controllers/operations-approver"
	updateobserver "control-plane-manager/internal/controllers/update-observer/controller"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/kube-api-rewriter/pkg/middleware/auth"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthProbeBindAddress   = "127.0.0.1:8095"
	metricsserverBindAddress = ":4296"

	pprofBindAddress = ":8097" // not used
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
	utilruntime.Must(updateobserverv1.AddToScheme(scheme))
}

type Manager struct {
	runtimeManager manager.Manager
}

func NewManager(ctx context.Context, pprof bool) (*Manager, error) {
	cfg := controllerruntime.GetConfigOrDie()
	controllerruntime.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	pprofAddr := ""
	if pprof {
		pprofAddr = pprofBindAddress
	}

	runtimeManager, err := controllerruntime.NewManager(cfg, controllerruntime.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: constants.CpcControllerName,
		BaseContext: func() context.Context {
			return ctx
		},
		Metrics: metricsserver.Options{
			BindAddress:    metricsserverBindAddress,
			SecureServing:  true,
			FilterProvider: metricsAuthFilterProvider,
		},
		HealthProbeBindAddress:  healthProbeBindAddress,
		PprofBindAddress:        pprofAddr,
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Pod{},
					&corev1.ConfigMap{},
				},
			},
		},
		Cache: cache.Options{
			ReaderFailOnMissingInformer: false,
			DefaultTransform:            cache.TransformStripManagedFields(),
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						constants.KubeSystemNamespace: {},
					},
				},
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						constants.KubeSystemNamespace: {},
					},
				},
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						constants.KubeSystemNamespace: {},
					},
				},
				&corev1.Node{}: {
					Transform: func(in any) (any, error) {
						node, ok := in.(*corev1.Node)
						if !ok {
							return in, nil
						}
						stripped := &corev1.Node{}
						stripped.Name = node.Name
						stripped.ResourceVersion = node.ResourceVersion
						stripped.UID = node.UID
						stripped.Labels = node.Labels
						stripped.Status = corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: node.Status.NodeInfo.KubeletVersion,
							},
						}
						return stripped, nil
					},
				},
				&updateobserverv1.NodeGroup{}: {
					Transform: func(in any) (any, error) {
						ng, ok := in.(*updateobserverv1.NodeGroup)
						if !ok {
							return in, nil
						}
						stripped := &updateobserverv1.NodeGroup{}
						stripped.Name = ng.Name
						stripped.ResourceVersion = ng.ResourceVersion
						stripped.UID = ng.UID
						stripped.Status.Ready = ng.Status.Ready
						return stripped, nil
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

	metricsStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithNewRegistry(),
		metricsstorage.WithLogger(log.Default().Named("metrics-storage")),
	)
	ctrlmetrics.Registry.MustRegister(metricsStorage.Collector())

	if err = controlplaneconfiguration.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register controlplane controller: %w", err)
	}

	if err = controlplanenode.Register(runtimeManager, metricsStorage); err != nil {
		return nil, fmt.Errorf("register control-plane-node controller: %w", err)
	}

	if err = controlplaneoperation.Register(runtimeManager, metricsStorage); err != nil {
		return nil, fmt.Errorf("register control-plane-operation controller: %w", err)
	}

	if err = operationsapprover.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register operations-approver controller: %w", err)
	}

	if err = updateobserver.RegisterController(runtimeManager); err != nil {
		return nil, fmt.Errorf("register update-observer controller: %w", err)
	}

	return &Manager{
		runtimeManager,
	}, nil
}

// metricsAuthFilterProvider mirrors kube-rbac-proxy sidecar behavior.
func metricsAuthFilterProvider(cfg *rest.Config, hc *http.Client) (metricsserver.Filter, error) {
	dsName := os.Getenv(constants.DaemonSetNameEnvVar)
	if dsName == "" {
		return nil, fmt.Errorf("metrics auth: %s env not set", constants.DaemonSetNameEnvVar)
	}

	kc, err := kubernetes.NewForConfigAndClient(cfg, hc)
	if err != nil {
		return nil, fmt.Errorf("metrics auth: build kube client: %w", err)
	}

	mw := auth.NewMiddlewareFromKubeClient(kc, auth.ResourceAttributes{
		Namespace:   constants.KubeSystemNamespace,
		Group:       "apps",
		Version:     "v1",
		Resource:    "daemonsets",
		Subresource: "prometheus-metrics",
		Name:        dsName,
	})

	return func(_ logr.Logger, h http.Handler) (http.Handler, error) {
		if h == nil {
			return nil, errors.New("metrics auth: nil handler")
		}
		return mw.Handler(h), nil
	}, nil
}

func (c *Manager) Start(ctx context.Context) error {
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			log.Fatal("failed to start runtime manager", log.Err(err))
		}
	}()
	log.Info("Control plane manager started")

	if ok := c.runtimeManager.GetCache().WaitForCacheSync(ctx); !ok {
		return fmt.Errorf("wait for cache sync")
	}
	log.Info("Cache synced")

	return nil
}
