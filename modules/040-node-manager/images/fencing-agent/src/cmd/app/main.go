/*
Copyright 2024 Flant JSC

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
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/controllers/grpc"
	"fencing-agent/internal/controllers/http"

	//"fencing-agent/internal/adapters/memberlist"
	//"fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/helper/logger/sl"
	"fencing-agent/internal/local"
	"fencing-agent/internal/usecase"
	"os/signal"
	"syscall"

	"fencing-agent/internal/config"
	"fencing-agent/internal/helper/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	var cfg config.Config
	cfg.MustLoad()

	log := logger.NewLogger(cfg.LogLevel)

	kubeClient, err := kubeclient.New(cfg.KubeClient, log, cfg.NodeName, cfg.NodeGroup)
	if err != nil {
		log.Error("failed to create KubernetesClient", sl.Err(err))
		panic(err)
	}

	ip, err := kubeClient.GetCurrentNodeIP(ctx)
	if err != nil {
		log.Error("failed to get current node IP", sl.Err(err))
		panic(err)
	}
	log.Info("current node IP", ip)
	//mblist, err := memberlist.New(cfg.Memberlist, log, ip, cfg.NodeName)
	mblist, err := local.NewMemberlist(log)
	if err != nil {
		log.Error("failed to create memberlist", sl.Err(err))
		panic(err)
	}

	ips, err := kubeClient.GetNodesIP(ctx)
	if err != nil {
		log.Error("failed to get nodes IPs", sl.Err(err))
		panic(err)
	}

	err = mblist.Start(ips)
	if err != nil {
		log.Error("failed to start memberlist", sl.Err(err))
		panic(err)
	}

	//softdog := watchdog.New(cfg.Watchdog.WatchdogDevice)
	var s []byte
	softdog := local.NewWatchdog(&s)

	totalNodes := len(ips)
	log.Info("total nodes", "totalNodes", totalNodes)

	decider := domain.NewQuorumDecider(totalNodes)

	fencingAgent := usecase.NewHealthMonitor(
		kubeClient,
		kubeClient,
		mblist,
		softdog,
		decider,
		kubeClient,
		log,
	)

	// get_nodes usecase
	nodesGetter := usecase.NewGetNodes(mblist)

	// eventbus usecase

	eventBus := usecase.NewEventsBus()

	grpcSrv := grpc.NewServer(eventBus, nodesGetter)

	grpcSrvRunner, err := grpc.NewRunner(cfg.GRPC, log, grpcSrv)
	if err != nil {
		panic(err)
	}

	healthzSrv := http.New(log, cfg.HealthProbeBindAddress)

	kubeClient.Start(ctx)
	fencingAgent.Start(ctx, cfg.Watchdog.WathcdogTimeout)
	healthzSrv.Start()
	grpcSrvRunner.Start()

	<-ctx.Done()

	healthzSrv.Stop()
	fencingAgent.Stop()
	kubeClient.Stop()
	mblist.Stop()
	// Start grpc server

}
