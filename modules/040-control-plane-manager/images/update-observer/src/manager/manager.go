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

package manager

import (
	"context"
	"fmt"
	"time"
	"update-observer/constant"
	"update-observer/controller"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthProbeBindAddress   = ":4264"
	pprofBindAddress         = ":4265"
	metricsserverBindAddress = ":4266"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

type Manager struct {
	manager.Manager
}

func NewManager(ctx context.Context, pprof bool) (*Manager, error) {
	cfg := controllerruntime.GetConfigOrDie()
	controllerruntime.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	// TODO: pprof flag?

	runtimeManager, err := controllerruntime.NewManager(cfg, controllerruntime.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: constant.ControllerName,
		BaseContext: func() context.Context {
			return ctx
		},
		Metrics: metricsserver.Options{
			BindAddress: metricsserverBindAddress,
		},
		HealthProbeBindAddress:  healthProbeBindAddress,
		PprofBindAddress:        "",
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
		Cache: cache.Options{
			ReaderFailOnMissingInformer: false,
			DefaultTransform:            cache.TransformStripManagedFields(),
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						constant.KubeSystemNamespace: {},
					},
				},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Node{},
					&corev1.Pod{},
					&corev1.ConfigMap{},
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
		runtimeManager,
	}, nil
}
