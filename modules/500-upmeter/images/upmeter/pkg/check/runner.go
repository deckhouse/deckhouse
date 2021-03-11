package check

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// Runner is a glue between the executor and a check.
type Runner struct {
	// defined by check

	probeRef  *ProbeRef
	checkName string
	checker   Checker
	period    time.Duration

	// defined by lifecycle

	send  chan<- Result
	state *state
}

func NewRunner(groupName, probeName, checkName string, period time.Duration, checker Checker) *Runner {
	ref := &ProbeRef{
		Group: groupName,
		Probe: probeName,
	}
	return &Runner{
		period:    period,
		probeRef:  ref,
		checkName: checkName,
		checker:   checker,
		state:     &state{firstRun: true},
	}
}

func (r *Runner) SendTo(send chan<- Result) {
	r.send = send
}

func (r *Runner) Id() string {
	if r.probeRef != nil {
		return r.probeRef.Id()
	}
	return ""
}

func (r *Runner) Run(start time.Time) {
	r.state.Start(start)

	go func() {
		status := StatusSuccess
		err := r.checker.Check()
		if err != nil {
			r.Logger().Errorf(err.Error())
			status = err.Status()
		}
		r.send <- r.Result(status)

		r.state.Stop()
	}()
}

func (r *Runner) ShouldRun(when time.Time) bool {
	return r.state.shouldRun(when, r.period)
}

func (r *Runner) Logger() *log.Entry {
	return log.
		WithField("group", r.probeRef.Group).
		WithField("probe", r.probeRef.Probe)
}

func (r *Runner) Result(status Status) Result {
	return NewResult(*r.probeRef, r.checkName, status)
}

type state struct {
	lastStart time.Time
	running   bool
	firstRun  bool
}

// ShouldRun checks that the probe can be run. Returns true if the probe is not
// running and its period passed
func (s *state) shouldRun(when time.Time, period time.Duration) bool {
	if s.running {
		return false
	}
	nextStart := s.lastStart.Add(period)
	return when.Equal(nextStart) || when.After(nextStart)
}

func (s *state) Start(t time.Time) {
	s.running = true
	s.lastStart = t
}

func (s *state) Stop() {
	s.running = false
	s.firstRun = false
}
