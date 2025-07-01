/*
Copyright 2025 Flant JSC

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

package checker

import (
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

	windowSize          int
	period              time.Duration
	freezeThreshold     time.Duration
	taskGrowthThreshold float64
}

func (c *convergeStatusChecker) Check() check.Error {
	pollResult, err := c.poller.Poll()
	if err != nil {
		return err
	}

	// deckhouse restarted, cleanup all previous c.history if startup converge was already done in this window,
	// so we don't get exceeding growth rate
	if c.startupConvergeDone && !pollResult.StartupConvergeDone && !c.historyCleaned {
		c.logger.Debugf("restart detected, cleaning up c.history")
		c.history = []status{}
		c.historyCleaned = true
	}

	if pollResult.StartupConvergeDone {
		c.logger.Debugf("startup converge was done, resetting historyCleaned marker")

		c.historyCleaned = false
		c.startupConvergeDone = true
	}

	c.logger.Debugf("poll ended %+v\n", pollResult)
	c.history = append(c.history, *pollResult)

	if len(c.history) > c.windowSize {
		c.logger.Debugf("history more than window size, removing oldest element")
		c.history = c.history[1:]
	}
	c.logger.Debugf("append to history %+v\n", c.history)

	if len(c.history) < 2 {
		// not enough data
		c.logger.Debugf("not enough data yet, %d\n", len(c.history))
		return nil
	}

	latest := c.history[len(c.history)-1]

	if latest.ConvergeWaitTask {
		c.logger.Debugf("queue is empty, skip check\n")

		// queue is empty, deckhouse is waiting for tasks
		return nil
	}

	start, end := c.history[0], latest
	startTasks := start.ConvergeInProgress + start.StartupConvergeInProgress
	endTasks := end.ConvergeInProgress + end.StartupConvergeInProgress
	c.logger.Debugf("start tasks %d, end tasks %d\n", startTasks, endTasks)

	duration := len(c.history) * int(c.period.Seconds())
	c.logger.Debugf("duration %d\n", duration)

	growthRate := float64((endTasks - startTasks) / duration)
	c.logger.Debugf("growth rate %f\n", growthRate)

	if growthRate > c.taskGrowthThreshold {
		c.logger.Debugf("growth rate %f exceeds task growth threshold %f\n", growthRate, c.taskGrowthThreshold)
		return check.ErrFail("deckhouse queue grows faster then expected")
	}

	// check for frozen queue
	allEqual := true
	ref := c.history[0].ConvergeInProgress + c.history[0].StartupConvergeInProgress
	for _, h := range c.history[1:] {
		cur := h.ConvergeInProgress + h.StartupConvergeInProgress
		if cur != ref {
			allEqual = false
			break
		}
	}

	if allEqual {
		frozenDuration := time.Duration(len(c.history)) * (time.Second * 60)
		if frozenDuration >= c.freezeThreshold {
			return check.ErrFail("queue size haven't changed in 5 minutes, queue is frozen")
		}
	}

	return nil
}
