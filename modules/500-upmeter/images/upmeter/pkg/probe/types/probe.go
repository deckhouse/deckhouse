package types

import (
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"
)

type Prober interface {
	Init() error
	Run(start time.Time) error
	ProbeId() string
	State() *ProbeState
	WithResultChan(ch chan ProbeResult)
	WithKubernetesClient(client kube.KubernetesClient)
}

type ProbeState struct {
	Running   bool
	StartedAt time.Time
	Period    time.Duration // Period between runs
	FirstRun  bool
}

// ShouldRun checks that the probe can be run. Returns true if the probe is not
// running and its period passed
func (ps *ProbeState) ShouldRun(now time.Time) bool {
	periodPassed := now.After(ps.StartedAt.Add(ps.Period))
	if !ps.Running && periodPassed {
		return true
	}
	return false
}

func (ps *ProbeState) Start(t time.Time) {
	ps.Running = true
	ps.StartedAt = t
}

func (ps *ProbeState) Stop() {
	ps.Running = false
	// Do not reset StartedAt to calculate delay.
	// Reset FirstRun
	if ps.FirstRun {
		ps.FirstRun = false
	}

}

type CommonProbe struct {
	ProbeState       *ProbeState
	ResultCh         chan ProbeResult
	KubernetesClient kube.KubernetesClient

	Period time.Duration

	ProbeRef *ProbeRef
	InitFn   func()
	RunFn    func()
}

func (c *CommonProbe) State() *ProbeState {
	return c.ProbeState
}

func (c *CommonProbe) Init() error {
	c.ProbeState = &ProbeState{
		FirstRun: true,
	}
	if c.Period > 0 {
		c.ProbeState.Period = c.Period
	}
	if c.InitFn != nil {
		c.InitFn()
	}
	return nil
}

func (c *CommonProbe) WithResultChan(ch chan ProbeResult) {
	c.ResultCh = ch
}

func (c *CommonProbe) WithKubernetesClient(client kube.KubernetesClient) {
	c.KubernetesClient = client
}

func (c *CommonProbe) ProbeId() string {
	if c.ProbeRef != nil {
		return c.ProbeRef.ProbeId()
	}
	return ""
}

func (c *CommonProbe) Run(start time.Time) error {
	//log.Infof("Run probe ")
	c.State().Start(start)

	go func() {
		if c.RunFn != nil {
			c.RunFn()
		}
		c.State().Stop()
	}()

	return nil
}

func (c *CommonProbe) LogEntry() *log.Entry {
	return log.WithField("group", c.ProbeRef.Group).
		WithField("probe", c.ProbeRef.Probe)
}

var _ Prober = &CommonProbe{}

// Result related methods
func (c *CommonProbe) Result(value interface{}) ProbeResult {
	return NewProbeResult(*c.ProbeRef, "_", value)
}

func (c *CommonProbe) CheckResult(checkName string, value interface{}) ProbeResult {
	return NewProbeResult(*c.ProbeRef, checkName, value)
}

func (c *CommonProbe) ResultForProbeRef(ref ProbeRef, value interface{}) ProbeResult {
	return NewProbeResult(ref, "_", value)
}

func (c *CommonProbe) CheckResultForProbeRef(ref ProbeRef, checkName string, value interface{}) ProbeResult {
	return NewProbeResult(ref, checkName, value)
}
