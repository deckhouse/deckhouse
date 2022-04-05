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

func NewLoader(access kubernetes.Access, logger *logrus.Logger) *Loader {
	return &Loader{
		access: access,
		logger: logger,
	}
}

type Loader struct {
	access kubernetes.Access
	logger *logrus.Logger

	groups []string
	probes []check.ProbeRef
}

func (l *Loader) Load(filter Filter) []*check.Runner {
	runConfigs := make([]runnerConfig, 0)

	runConfigs = append(runConfigs, initSynthetic(l.access, l.logger)...)
	runConfigs = append(runConfigs, initControlPlane(l.access)...)
	runConfigs = append(runConfigs, initMonitoringAndAutoscaling(l.access)...)
	runConfigs = append(runConfigs, initScaling(l.access)...)
	runConfigs = append(runConfigs, initLoadBalancing(l.access)...)
	runConfigs = append(runConfigs, initDeckhouse(l.access, l.logger)...)

	runners := make([]*check.Runner, 0)
	for _, rc := range runConfigs {
		if !filter.Enabled(rc.Ref()) {
			continue
		}

		runnerLogger := l.logger.WithFields(map[string]interface{}{
			"group": rc.group,
			"probe": rc.probe,
			"check": rc.check,
		})

		runner := check.NewRunner(rc.group, rc.probe, rc.check, rc.period, rc.config.Checker(), runnerLogger)

		runners = append(runners, runner)
		l.groups = append(l.groups, rc.group)
		l.probes = append(l.probes, runner.ProbeRef())

		l.logger.Infof("Register probe %s", runner.ProbeRef().Id())
	}

	return runners
}

func (l *Loader) Groups() []string {
	return l.groups
}

func (l *Loader) Probes() []check.ProbeRef {
	return l.probes
}

type runnerConfig struct {
	group  string
	probe  string
	check  string
	period time.Duration
	config checker.Config
}

func (rc runnerConfig) Ref() check.ProbeRef {
	return check.ProbeRef{Group: rc.group, Probe: rc.probe}
}

type Filter interface {
	Enabled(ref check.ProbeRef) bool
}
