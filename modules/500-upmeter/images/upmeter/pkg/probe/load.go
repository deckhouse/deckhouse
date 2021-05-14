package probe

import (
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func Load(access kubernetes.Access) []*check.Runner {
	runConfigs := make([]runnerConfig, 0)

	runConfigs = append(runConfigs, Synthetic()...)
	runConfigs = append(runConfigs, ControlPlane(access)...)
	runConfigs = append(runConfigs, MonitoringAndAutoscaling(access)...)

	runners := make([]*check.Runner, 0)
	for _, rc := range runConfigs {
		runner := check.NewRunner(rc.group, rc.probe, rc.check, rc.period, rc.config.Checker())
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
