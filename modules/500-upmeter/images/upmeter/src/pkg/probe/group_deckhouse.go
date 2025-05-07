/*
Copyright 2021 Flant JSC

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

package probe

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/hookprobe"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/probe/run"
)

type probeConfig struct {
	WindowSize                  int
	FreezeThreshold             time.Duration
	AllowedTasksPerTimeInterval float64
	TaskGrowthThreshold         float64
}

var (
	config          probeConfig
	probeConfigOnce sync.Once
)

func parseConfigFromEnv(logger *logrus.Logger, period time.Duration) probeConfig {
	const prefix = "UPMETER_AGENT_DECKHOUSE_"

	defaultWindowSize := 5
	defaultFreezeThreshold := 5 * time.Minute
	defaultAllowedTasks := 10.0

	cfg := probeConfig{}

	if val := os.Getenv(prefix + "WINDOW_SIZE"); val != "" {
		if v, err := strconv.Atoi(val); err == nil && v > 0 {
			cfg.WindowSize = v
		} else {
			logger.Errorf("invalid %sWINDOW_SIZE=%q, using default %d\n", prefix, val, defaultWindowSize)
			cfg.WindowSize = defaultWindowSize
		}
	} else {
		cfg.WindowSize = defaultWindowSize
	}

	if val := os.Getenv(prefix + "FREEZE_THRESHOLD"); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			cfg.FreezeThreshold = d
		} else {
			logger.Errorf("invalid %sFREEZE_THRESHOLD=%q, using default %s\n", prefix, val, defaultFreezeThreshold)
			cfg.FreezeThreshold = defaultFreezeThreshold
		}
	} else {
		cfg.FreezeThreshold = defaultFreezeThreshold
	}

	if val := os.Getenv(prefix + "ALLOWED_TASKS_PER_INTERVAL"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 {
			cfg.AllowedTasksPerTimeInterval = f
		} else {
			logger.Errorf("invalid %sALLOWED_TASKS_PER_INTERVAL=%q, using default %.2f\n", prefix, val, defaultAllowedTasks)
			cfg.AllowedTasksPerTimeInterval = defaultAllowedTasks
		}
	} else {
		cfg.AllowedTasksPerTimeInterval = defaultAllowedTasks
	}

	cfg.TaskGrowthThreshold = cfg.AllowedTasksPerTimeInterval / float64(cfg.WindowSize*int(period.Seconds()))

	return cfg
}

func initDeckhouse(access kubernetes.Access, preflight checker.Doer, logger *logrus.Logger) []runnerConfig {
	const (
		groupDeckhouse      = "deckhouse"
		controlPlaneTimeout = 5 * time.Second
		period              = 30 * time.Second
	)

	probeConfigOnce.Do(func() {
		config = parseConfigFromEnv(logger, period)
	})

	logEntry := logrus.NewEntry(logger).WithField("group", groupDeckhouse)
	monitor := hookprobe.NewMonitor(access.Kubernetes(), logEntry)
	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	return []runnerConfig{
		{
			group:  groupDeckhouse,
			probe:  "cluster-configuration",
			check:  "_",
			period: period,
			config: &checker.D8ClusterConfiguration{
				// deckhouse
				DeckhouseNamespace:     "d8-system",
				DeckhouseLabelSelector: "app=deckhouse",

				// CR
				CustomResourceName: run.ID(),
				Monitor:            monitor,

				Access:              access,
				PreflightChecker:    controlPlanePinger,
				PodAccessTimeout:    5 * time.Second,
				ObjectChangeTimeout: 5 * time.Second,

				WindowSize:                  config.WindowSize,
				FreezeThreshold:             config.FreezeThreshold,
				AllowedTasksPerTimeInterval: config.AllowedTasksPerTimeInterval,
				TaskGrowthThreshold:         config.TaskGrowthThreshold,
				Logger:                      logEntry.WithField("probe", "cluster-configuration"),
			},
		},
	}
}
