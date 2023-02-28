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

package probe

import (
	"time"

	"github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/hookprobe"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/probe/run"
)

func initDeckhouse(access kubernetes.Access, preflight checker.Doer, logger *logrus.Logger) []runnerConfig {
	const (
		groupDeckhouse      = "deckhouse"
		controlPlaneTimeout = 5 * time.Second
	)

	logEntry := logrus.NewEntry(logger).WithField("group", groupDeckhouse)
	monitor := hookprobe.NewMonitor(access.Kubernetes(), logEntry)
	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	return []runnerConfig{
		{
			group:  groupDeckhouse,
			probe:  "cluster-configuration",
			check:  "_",
			period: time.Minute,
			config: &checker.D8ClusterConfiguration{
				// deckhouse
				DeckhouseNamespace:     "d8-system",
				DeckhouseLabelSelector: "app=deckhouse",

				// CR
				CustomResourceName: run.ID(),
				Monitor:            monitor,

				Access:           access,
				PreflightChecker: controlPlanePinger,

				DeckhouseReadinessTimeout: 20 * time.Minute,
				PodAccessTimeout:          5 * time.Second,
				ObjectChangeTimeout:       5 * time.Second,

				Logger: logEntry.WithField("probe", "cluster-configuration"),
			},
		},
	}
}
