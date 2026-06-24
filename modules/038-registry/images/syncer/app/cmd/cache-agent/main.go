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

// Binary cache-agent runs registry-cache replication (follower mirror+prune of
// the leader) + garbage collection, with K8s leader election selecting the
// single source of truth.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"syncer/pkg/agent"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfgPath := envDefault("AGENT_CONFIG", "/agent/config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		logger.Error("read agent config", "path", cfgPath, "error", err)
		os.Exit(1)
	}
	cfg, err := agent.LoadConfig(raw)
	if err != nil {
		logger.Error("load agent config", "error", err)
		os.Exit(1)
	}

	podName := os.Getenv("POD_NAME")
	namespace := envDefault("POD_NAMESPACE", "d8-system")
	if podName == "" {
		logger.Error("POD_NAME not set")
		os.Exit(1)
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		logger.Error("in-cluster config", "error", err)
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		logger.Error("k8s client", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var isLeader atomic.Bool
	a := agent.New(logger, cfg, &isLeader)
	go a.RunLoops(ctx)

	lm := agent.NewLeaderManager(client, logger, namespace, podName, "registry-cache-leader", &isLeader)

	// Re-enter election after demotion: leaderelection.Run returns when this pod
	// loses the lease, but the pod stays alive as a follower (RunLoops keeps
	// mirroring, now that isLeader is false). Exit only on context cancellation.
	for {
		lm.Run(ctx) // blocks while participating; returns on demotion or ctx cancel
		if ctx.Err() != nil {
			return
		}
		logger.Info("re-entering leader election after demotion")
	}
}

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
