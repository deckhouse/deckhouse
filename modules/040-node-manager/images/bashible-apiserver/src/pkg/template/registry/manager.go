// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	Namespace = "d8-system"
)

func SetupAndStartManager(ctx context.Context) chan RegistryDataWithHash {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("unable to create cluster config: %w", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:          false,
		GracefulShutdownTimeout: &[]time.Duration{10 * time.Second}[0],
		Namespace:               Namespace,
	})
	if err != nil {
		klog.Fatalf("unable to set up registry state controller manager: %w", err)
	}

	rsc := RegistryStateController{
		Namespace: Namespace,
	}
	resultCh, err := rsc.SetupWithManager(ctx, mgr)
	if err != nil {
		klog.Fatalf("unable to set up registry state controller: %w", err)
	}

	go func() {
		if err := mgr.Start(ctx); err != nil {
			klog.Fatalf("unable to start manager: %w", err)
		}
	}()
	return resultCh
}
