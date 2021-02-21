package checks

import (
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"
)

type Probe struct {
	ResultCh chan Result
	Period   time.Duration
	Ref      *ProbeRef
	InitFn   func()
	RunFn    func()
	state    *State

	KubernetesClient    kube.KubernetesClient
	ServiceAccountToken string
}

func (p *Probe) State() *State {
	return p.state
}

func (p *Probe) Init() error {
	p.state = &State{
		FirstRun: true,
	}
	if p.Period > 0 {
		p.state.Period = p.Period
	}
	if p.InitFn != nil {
		p.InitFn()
	}
	return nil
}

func (p *Probe) WithResultChan(ch chan Result) {
	p.ResultCh = ch
}

func (p *Probe) WithKubernetesClient(client kube.KubernetesClient) {
	p.KubernetesClient = client
}

func (p *Probe) WithServiceAccountToken(token string) {
	p.ServiceAccountToken = token
}

func (p *Probe) Id() string {
	if p.Ref != nil {
		return p.Ref.Id()
	}
	return ""
}

func (p *Probe) Run(start time.Time) error {
	p.State().Start(start)

	go func() {
		if p.RunFn != nil {
			p.RunFn()
		}
		p.State().Stop()
	}()

	return nil
}

func (p *Probe) LogEntry() *log.Entry {
	return log.
		WithField("group", p.Ref.Group).
		WithField("probe", p.Ref.Probe)
}

// Result related methods
func (p *Probe) Result(value interface{}) Result {
	return NewResult(*p.Ref, "_", value)
}

func (p *Probe) CheckResult(checkName string, value interface{}) Result {
	return NewResult(*p.Ref, checkName, value)
}

type State struct {
	Running   bool
	StartedAt time.Time
	Period    time.Duration // Period between runs
	FirstRun  bool
}

// ShouldRun checks that the probe can be run. Returns true if the probe is not
// running and its period passed
func (ps *State) ShouldRun(now time.Time) bool {
	periodPassed := now.After(ps.StartedAt.Add(ps.Period))
	if !ps.Running && periodPassed {
		return true
	}
	return false
}

func (ps *State) Start(t time.Time) {
	ps.Running = true
	ps.StartedAt = t
}

func (ps *State) Stop() {
	ps.Running = false
	// Do not reset StartedAt to calculate delay.
	// Reset FirstRun
	if ps.FirstRun {
		ps.FirstRun = false
	}

}
