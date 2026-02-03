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
	"context"
	"fmt"
	"time"
	
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/controllers/controlplane"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthProbeBindAddress   = ":8095"
	pprofBindAddress         = ":8096"
	metricsserverBindAddress = ":8097"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controlplanev1alpha1.AddToScheme(scheme))
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
		Scheme:         scheme,
		LeaderElection: true,
		BaseContext: func() context.Context {
			return ctx
		},
		Metrics: metricsserver.Options{
			BindAddress: metricsserverBindAddress,
		},
		HealthProbeBindAddress:  healthProbeBindAddress,
		PprofBindAddress:        pprofAddr,
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
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

	if err = controlplane.Register(runtimeManager); err != nil {
		return nil, fmt.Errorf("register controlplane controller: %w", err)
	}

	return &Manager{
		runtimeManager,
	}, nil
}

func (c *Manager) Start(ctx context.Context) error {
	go func() {
		if err := c.runtimeManager.Start(ctx); err != nil {
			klog.Fatalf("failed to start runtime manager: %v", err)
		}
	}()
	klog.Info("Control plane manager started")

	if ok := c.runtimeManager.GetCache().WaitForCacheSync(ctx); !ok {
		return fmt.Errorf("wait for cache sync")
	}
	klog.Info("Cache synced")

	return nil
}
