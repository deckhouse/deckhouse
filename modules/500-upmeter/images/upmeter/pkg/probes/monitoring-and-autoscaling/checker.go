package monitoring_and_autoscaling

import (
	"time"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes/util"
)

// Checker defines the common interface for check operations
//
// Do not re-use checkers, always create new ones. Usually checkers are a stateful composition of other checkers.
// They are stateful due to BusyWith method. See SequentialChecker as an example.
type Checker interface {
	// Check does the actual job to determine the result. Returns nil if everything is ok.
	Check() checks.Error

	// BusyWith describes what the check is doing. Used in logging and possibly other details of the
	// probe flow.
	BusyWith() string
}

// timeoutChecker wraps a checker with timer. If the timer finishes before the wrapped checker,
// unknown result error is returned.
type timeoutChecker struct {
	checker Checker
	timeout time.Duration
}

func withTimeout(checker Checker, timeout time.Duration) Checker {
	return &timeoutChecker{
		checker: checker,
		timeout: timeout,
	}
}

func (c *timeoutChecker) Check() checks.Error {
	var err checks.Error
	util.DoWithTimer(c.timeout,
		func() {
			err = c.checker.Check()
		},
		func() {
			err = checks.ErrUnknownResult("timed out: %s", c.checker.BusyWith())
		},
	)
	return err
}

func (c *timeoutChecker) BusyWith() string {
	return c.checker.BusyWith()
}

// SequentialChecker wraps the sequence of checkers and returns check error that occurs first and stops running
// all next checkers. Since it maintains the state for BusyWith, the sequential checker should not be reused but rather
// created again.
type SequentialChecker struct {
	checkers []Checker
	current  int
}

func NewSequentialChecker(checkers ...Checker) Checker {
	return &SequentialChecker{checkers: checkers}
}

func (c *SequentialChecker) Check() checks.Error {
	for i, checker := range c.checkers {
		c.current = i
		err := checker.Check()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SequentialChecker) BusyWith() string {
	return c.checkers[c.current].BusyWith()
}
