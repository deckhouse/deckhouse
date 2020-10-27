package types

import (
	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"
)

type Prober interface {
	Init() error
	Run(start int64) error
	ProbeId() string
	State() *ProbeState
	WithResultChan(ch chan ProbeResult)
	WithKubernetesClient(client kube.KubernetesClient)
}

type ProbeState struct {
	Running   bool
	StartedAt int64
	Period    int64 // Period between runs
}

// probe should run if it is not running and period is reached
func (ps *ProbeState) ShouldRun(now int64) bool {
	if !ps.Running && now-ps.StartedAt >= ps.Period {
		return true
	}
	return false
}

func (ps *ProbeState) Start(tm int64) {
	ps.Running = true
	ps.StartedAt = tm
}

func (ps *ProbeState) Stop() {
	ps.Running = false
	// Do not reset StartedAt to calculate delay.
}

type CommonProbe struct {
	ProbeState       *ProbeState
	ResultCh         chan ProbeResult
	KubernetesClient kube.KubernetesClient

	Period int64

	ProbeRef *ProbeRef
	InitFn   func()
	RunFn    func(start int64)
}

func (c *CommonProbe) State() *ProbeState {
	return c.ProbeState
}

func (c *CommonProbe) Init() error {
	c.ProbeState = &ProbeState{}
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

func (c *CommonProbe) Run(start int64) error {
	//log.Infof("Run probe ")
	c.State().Start(start)

	go func() {
		if c.RunFn != nil {
			c.RunFn(start)
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
