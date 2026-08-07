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
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/fencingstate"
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/agent"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/profile"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// resolveIdentityTimeout and profileLoadTimeout bound the worst-case startup
// blocking (30s + 15s = 45s) before the health server starts, since it starts
// only after both identity resolution and profile load finish. That total
// must stay under the liveness kill window (~55s: initialDelay 15s + period
// 20s x failureThreshold 3) or the kubelet kills the pod before it ever
// answers a liveness probe.
const (
	resolveIdentityTimeout = 30 * time.Second
	profileLoadTimeout     = 15 * time.Second
)

func main() {
	logger := newLogger()

	if err := run(logger); err != nil {
		logger.Error("fencing-agent failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg := &config.Config{}

	if err := cfg.Load(); err != nil {
		return err
	}

	logger.SetLevel(log.LogLevelFromStr(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	restCfg, err := kubeclient.NewRestConfig()
	if err != nil {
		return fmt.Errorf("build kubernetes rest config: %w", err)
	}

	var deps agent.Deps

	deps.K8sClient, err = kubeclient.New(restCfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}

	deps.FencingClient, err = fencingstate.NewClient(restCfg)
	if err != nil {
		return fmt.Errorf("create FencingFailedNodeState client: %w", err)
	}

	resolveCtx, cancel := context.WithTimeout(ctx, resolveIdentityTimeout)
	defer cancel()

	identity, err := resolveIdentity(resolveCtx, deps.K8sClient, cfg.NodeName, logger)
	if err != nil {
		return fmt.Errorf("resolve node identity: %w", err)
	}

	cfg.NodeUID = identity.UID

	profileCtx, cancelProfile := context.WithTimeout(ctx, profileLoadTimeout)
	defer cancelProfile()

	// Exiting on any profile failure is deliberate (fail closed): the pod
	// restart loop is the retry mechanism, CrashLoopBackOff is the visibility.
	sla, err := profile.Load(profileCtx, fencingstate.NewProfiles(deps.FencingClient), v1alpha1.ProfileName(cfg.ProfileRefName), logger)
	if err != nil {
		return fmt.Errorf("load SLA profile: %w", err)
	}

	return agent.New(cfg, deps, identity, sla, logger).Run(ctx)
}

// resolveIdentity retries: a cloud controller may populate the InternalIP a few
// seconds after the pod starts, and a single-shot failure would crash-loop it.
func resolveIdentity(ctx context.Context, k8s kubernetes.Interface, nodeName string, logger *log.Logger) (domain.NodeIdentity, error) {
	const retryInterval = 2 * time.Second

	for {
		identity, err := kubeclient.ResolveIdentity(ctx, k8s, nodeName)
		if err == nil {
			return identity, nil
		}

		logger.Warn("resolve node identity failed, retrying", "error", err, "retry_interval", retryInterval.String())

		select {
		case <-ctx.Done():
			return domain.NodeIdentity{}, err
		case <-time.After(retryInterval):
		}
	}
}

func newLogger() *log.Logger {
	return log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithHandlerType(log.JSONHandlerType),
	)
}
