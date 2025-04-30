package checker

import (
	"fmt"
	"time"

	"d8.io/upmeter/pkg/check"
	"github.com/sirupsen/logrus"
)

type convergeStatusChecker struct {
	history             []status
	startupConvergeDone bool
	historyCleaned      bool
	poller              poller
	logger              *logrus.Entry
}

const (
	windowSize          = 6
	taskGrowthThreshold = 0.01
	freezeThreshold     = 5 * time.Minute
)

func (c *convergeStatusChecker) Check() check.Error {
	pollResult, err := c.poller.Poll()
	if err != nil {
		return err
	}

	// deckhouse restarted, cleanup all previous c.history if startup converge was already done in this window,
	// so we don't get exceeding growth rate
	if c.startupConvergeDone && !pollResult.StartupConvergeDone && !c.historyCleaned {
		fmt.Printf("restart detected, cleaning up c.history")
		c.history = []status{}
		c.historyCleaned = true
	}

	if pollResult.StartupConvergeDone {
		fmt.Printf("startup converge was done, resetting historyCleaned marker")

		c.historyCleaned = false
		c.startupConvergeDone = true
	}

	fmt.Printf("poll ended %+v\n", pollResult)
	c.history = append(c.history, *pollResult)

	if len(c.history) > windowSize {
		fmt.Printf("history more than window size, removing oldest element")
		c.history = c.history[1:]
	}
	fmt.Printf("append to history %+v\n", c.history)

	if len(c.history) < 2 {
		// not enough data
		fmt.Printf("not enough data yet, %d\n", len(c.history))
		return nil
	}

	latest := c.history[len(c.history)-1]

	if latest.ConvergeWaitTask {
		fmt.Printf("queue is empty, skip check\n")

		// queue is empty, deckhouse is waiting for tasks
		return nil
	}

	start, end := c.history[0], latest
	startTasks := toInt(start.ConvergeInProgress) + toInt(start.StartupConvergeInProgress)
	endTasks := toInt(end.ConvergeInProgress) + toInt(end.StartupConvergeInProgress)
	fmt.Printf("start tasks %d, end tasks %d\n", startTasks, endTasks)

	duration := time.Duration(len(c.history)) * (time.Second * 60)
	fmt.Printf("duration %d\n", duration)

	growthRate := float64(endTasks-startTasks) / duration.Seconds()
	fmt.Printf("growth rate %d\n", growthRate)

	if growthRate > taskGrowthThreshold {
		fmt.Printf("growth rate exceeds task growth threshold %d\n", growthRate)
		return check.ErrFail("growth rate exceeds task growth threshold")
	}

	// check for frozen queue
	allEqual := true
	ref := toInt(c.history[0].ConvergeInProgress) + toInt(c.history[0].StartupConvergeInProgress)
	for _, h := range c.history[1:] {
		cur := toInt(h.ConvergeInProgress) + toInt(h.StartupConvergeInProgress)
		if cur != ref {
			allEqual = false
			break
		}
	}

	if allEqual {
		frozenDuration := time.Duration(len(c.history)) * (time.Second * 60)
		if frozenDuration >= freezeThreshold {
			return check.ErrFail("queue size haven't changed in 5 minutes, possibly frozen")
		}
	}

	return nil
}

func toInt(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}
