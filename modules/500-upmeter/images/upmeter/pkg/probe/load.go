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
	"time"

	"github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func Load(access kubernetes.Access, logger *logrus.Logger) []*check.Runner {
	runConfigs := make([]runnerConfig, 0)

	runConfigs = append(runConfigs, initSynthetic(access, logger)...)
	runConfigs = append(runConfigs, initControlPlane(access)...)
	runConfigs = append(runConfigs, initMonitoringAndAutoscaling(access)...)
	runConfigs = append(runConfigs, initScaling(access)...)
	runConfigs = append(runConfigs, initLoadBalancing(access)...)
	runConfigs = append(runConfigs, initDeckhouse(access, logger)...)

	runners := make([]*check.Runner, 0)
	for _, rc := range runConfigs {
		runnerLogger := logger.WithFields(map[string]interface{}{
			"group": rc.group,
			"probe": rc.probe,
			"check": rc.check,
		})
		runner := check.NewRunner(rc.group, rc.probe, rc.check, rc.period, rc.config.Checker(), runnerLogger)
		runners = append(runners, runner)
	}
	return runners
}

type runnerConfig struct {
	group  string
	probe  string
	check  string
	period time.Duration
	config checker.Config
}
