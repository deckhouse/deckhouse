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

package apiserver

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

const (
	CtrlManagerNamespace      = "d8-system"
	CtrlManagerHealthAddr     = ":8097"
	CtrlManagerLeaderElection = false
)

func NewCtrlManager() (ctrl.Manager, error) {
	cfg, err := ctrlManagerClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create cluster config: %w", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:         CtrlManagerLeaderElection,
		HealthProbeBindAddress: CtrlManagerHealthAddr,
		Namespace:              CtrlManagerNamespace,
		Logger:                 klog.NewKlogr(),
	})

	if err != nil {
		return nil, fmt.Errorf("unable to create ctrl manager: %w", err)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up ready check: %w", err)
	}
	return mgr, err
}

func ctrlManagerClusterConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}
