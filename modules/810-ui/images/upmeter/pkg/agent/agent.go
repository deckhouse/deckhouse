/*
Copyright 2023 Flant JSC

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

package agent

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/agent/scheduler"
	"d8.io/upmeter/pkg/agent/sender"
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/node"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/registry"
)

type Agent struct {
	config     *Config
	kubeConfig *kubernetes.Config

	logger *log.Logger

	sender    *sender.Sender
	scheduler *scheduler.Scheduler
}

type Config struct {
	Interval     time.Duration
	ClientConfig *sender.ClientConfig
	DatabasePath string
	UserAgent    string

	DisabledProbes []string
	DynamicProbes  *DynamicProbesConfig
}

type DynamicProbesConfig struct {
	IngressControllers []string
	NodeGroups         []string
	Zones              []string
	ZonePrefix         string
}

func NewConfig() *Config {
	return &Config{
		ClientConfig:  &sender.ClientConfig{},
		DynamicProbes: &DynamicProbesConfig{},
	}
}

func New(config *Config, kubeConfig *kubernetes.Config, logger *log.Logger) *Agent {
	return &Agent{
		config:     config,
		kubeConfig: kubeConfig,
		logger:     logger,
	}
}

func (a *Agent) Start(ctx context.Context) error {
	// Initialize kube client from kubeconfig and service account token from filesystem.
	kubeAccess := &kubernetes.Accessor{}
	err := kubeAccess.Init(a.kubeConfig, a.config.UserAgent)
	if err != nil {
		return fmt.Errorf("cannot init access to Kubernetes cluster: %v", err)
	}

	// Probe registry
	ftr := probe.NewProbeFilter(a.config.DisabledProbes)
	dynamicConfig := probe.DynamicConfig{
		IngressNginxControllers: a.config.DynamicProbes.IngressControllers,
		NodeGroups:              a.config.DynamicProbes.NodeGroups,
		Zones:                   a.config.DynamicProbes.Zones,
		ZonePrefix:              a.config.DynamicProbes.ZonePrefix,
	}

	nodeMon := node.NewMonitor(kubeAccess.Kubernetes(), log.NewEntry(a.logger))
	if err := nodeMon.Start(ctx); err != nil {
		return fmt.Errorf("starting node monitor: %v", err)
	}

	// The preflight interval is chosen as the smallest period among probe runs which use the
	// preflight check.
	preflightInterval := 5 * time.Second
	controlPlanePreflight := checker.NewK8sVersionGetter(kubeAccess, preflightInterval)
	controlPlanePreflight.Start()

	runnerLoader := probe.NewLoader(ftr, kubeAccess, nodeMon, dynamicConfig, controlPlanePreflight, a.logger)
	calcLoader := calculated.NewLoader(ftr, a.logger)
	registry := registry.New(runnerLoader, calcLoader)

	// Database connection with pool
	dbctx, err := db.Connect(a.config.DatabasePath, dbcontext.DefaultConnectionOptions())
	if err != nil {
		return fmt.Errorf("cannot connect to database: %v", err)
	}

	ch := make(chan []check.Episode)

	client := sender.NewClient(a.config.ClientConfig)
	storage := sender.NewStorage(dbctx)

	a.sender = sender.New(client, ch, storage, a.config.Interval)
	a.scheduler = scheduler.New(registry, ch)

	a.sender.Start()
	a.scheduler.Start()

	return nil
}

func (a *Agent) Stop() error {
	a.scheduler.Stop()
	a.sender.Stop()
	return nil
}
