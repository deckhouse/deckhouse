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
	"testing"
	"time"

	"d8.io/upmeter/pkg/check"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type fakePoller struct {
	results []*status
	err     check.Error
	index   int
}

func (f *fakePoller) Poll() (*status, check.Error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.index >= len(f.results) {
		return f.results[len(f.results)-1], nil
	}
	res := f.results[f.index]
	f.index++
	return res, nil
}

func makeChecker(poller poller) *convergeStatusChecker {
	l := logrus.New().WithField("unit", "test")
	l.Logger.SetLevel(logrus.DebugLevel)
	return &convergeStatusChecker{
		poller: poller,
		logger: l,

		windowSize:          5,
		freezeThreshold:     5 * time.Minute,
		taskGrowthThreshold: 10.0 / (5 * 30),

		period: time.Second * 30,
	}
}

func TestCheck_NotEnoughData(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true},
		},
	})
	err := c.Check()
	assert.Nil(t, err)
}

func TestCheck_NormalGrowth(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true, ConvergeInProgress: 1},
			{StartupConvergeDone: true, ConvergeInProgress: 2},
			{StartupConvergeDone: true, ConvergeInProgress: 3},
			{StartupConvergeDone: true, ConvergeInProgress: 4},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 6},
		},
	})
	for i := 0; i < 6; i++ {
		err := c.Check()
		assert.Nil(t, err)
	}
}

func TestCheck_ExceedsGrowthThreshold(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true, ConvergeInProgress: 1},
			{StartupConvergeDone: true, ConvergeInProgress: 20},
			{StartupConvergeDone: true, ConvergeInProgress: 30},
			{StartupConvergeDone: true, ConvergeInProgress: 40},
			{StartupConvergeDone: true, ConvergeInProgress: 50},
			{StartupConvergeDone: true, ConvergeInProgress: 600},
		},
	})
	var lastErr check.Error
	for i := 0; i < 6; i++ {
		lastErr = c.Check()
	}

	assert.Equal(t, "growth rate exceeds task growth threshold", lastErr.Error())
}

func TestCheck_FrozenQueue(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
			{StartupConvergeDone: true, ConvergeInProgress: 5},
		},
	})
	var lastErr check.Error
	for i := 0; i < 6; i++ {
		lastErr = c.Check()
	}
	assert.NotNil(t, lastErr)
	assert.Equal(t, "queue size haven't changed in 5 minutes, possibly frozen", lastErr.Error())
}

func TestCheck_QueueWait_NoError(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true, ConvergeWaitTask: true},
			{StartupConvergeDone: true, ConvergeWaitTask: true},
		},
	})
	for i := 0; i < 2; i++ {
		err := c.Check()
		assert.Nil(t, err)
	}
}

func TestCheck_RestartDetected_ClearsHistory(t *testing.T) {
	c := makeChecker(&fakePoller{
		results: []*status{
			{StartupConvergeDone: true, ConvergeInProgress: 1},
			{StartupConvergeDone: true, ConvergeInProgress: 1},
			{StartupConvergeDone: true, ConvergeInProgress: 2},
			{StartupConvergeDone: true, ConvergeInProgress: 3},
			{StartupConvergeDone: true, ConvergeInProgress: 4},
			{StartupConvergeDone: false, ConvergeInProgress: 0},
		},
	})
	for i := 0; i < 6; i++ {
		_ = c.Check()
	}
	assert.True(t, c.historyCleaned)
}
