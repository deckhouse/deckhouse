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
	"control-plane-manager/internal/constants"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	healthProbeBindAddress   = "127.0.0.1:8095"
	metricsserverBindAddress = ":4296"
)

type builder struct {
	configurator
}

func NewBuilder(ctType constants.ControlPlaneType) (builder, error) {
	var c configurator

	switch ctType {
	case constants.ControlPlaneTypeNormal:
		c = &normalConfigurator{}
	case constants.ControlPlaneTypeVirtual:
		c = &virtualConfigurator{}
	default:
		return builder{}, fmt.Errorf("unsupported control plane manager mode %q", ctType)
	}

	return builder{configurator: c}, nil
}

func (b builder) Build(ctx context.Context) (*Manager, error) {
	controllerruntime.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	runtimeManager, err := b.buildRuntimeManager(b.buildOptions(ctx))
	if err != nil {
		return nil, fmt.Errorf("build runtime manager: %w", err)
	}

	return &Manager{
		runtimeManager: runtimeManager,
	}, nil
}

func (b builder) buildOptions(ctx context.Context) controllerruntime.Options {
	opts := controllerruntime.Options{
		Scheme:           scheme,
		LeaderElection:   getLeaderElection(),
		LeaderElectionID: constants.ControlPlaneManagerName,
		BaseContext: func() context.Context {
			return ctx
		},
		Metrics: metricsserver.Options{
			BindAddress:   metricsserverBindAddress,
			SecureServing: true,
		},
		HealthProbeBindAddress:  healthProbeBindAddress,
		PprofBindAddress:        "",
		GracefulShutdownTimeout: ptr.To(10 * time.Second),
	}

	b.configurator.configureOptions(&opts)

	return opts
}

func (b builder) buildRuntimeManager(opts controllerruntime.Options) (manager.Manager, error) {
	runtimeManager, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), opts)
	if err != nil {
		return nil, fmt.Errorf("create controller runtime manager: %w", err)
	}

	if err := runtimeManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add health check: %w", err)
	}
	if err := runtimeManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add ready check: %w", err)
	}

	if err := b.configurator.configureRuntimeManager(runtimeManager); err != nil {
		return nil, fmt.Errorf("configurate controller runtime manager: %w", err)
	}

	return runtimeManager, nil
}

func getLeaderElection() bool {
	// Leader election is needed only when multiple cpm pods may compete for
	// cluster-wide work. cpm controllers that touch a specific master (static
	// pod manifest writes, per-master CRs) are already partitioned by
	// `spec.nodeName` (see controlPlanePodPredicate, operations-approver, ...),
	// so on a single-master cluster there is exactly one pod and nothing to
	// coordinate. Toggle by the LEADER_ELECTION env (set by helm based on
	// master replica count): true on HA (>1 master), false on single-master.
	//
	// Why this matters during bootstrap: cpm's very first reconcile rewrites
	// the etcd and kube-apiserver static-pod manifests, kubelet restarts those
	// pods, and the local kubernetes-api-proxy returns RST for the ~60s of the
	// restart cycle. With LE on, lease renewal exceeded RenewDeadline and the
	// manager exited with "leader election lost" — kubelet restarted cpm,
	// progress was lost, cpm redid SyncManifests producing an extra static-pod
	// restart and a knock-on cilium endpoint regeneration. With LE off (the
	// single-master case) the renewal pressure is gone entirely.
	return os.Getenv("LEADER_ELECTION") == "true"
}
