// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooklane_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type lanesSuite struct {
	suite.Suite

	calls  *calls
	clock  *clock
	logs   *logs
	router *hooklane.Router
}

func TestLanes(t *testing.T) {
	suite.Run(t, new(lanesSuite))
}

func (s *lanesSuite) SetupTest() {
	s.calls = newCalls()
	s.clock = newClock(s.calls)
	s.logs = &logs{calls: s.calls}
	s.router = hooklane.NewRouter(pauser{calls: s.calls}, failures{calls: s.calls}, s.clock,
		log.NewLogger(log.WithOutput(s.logs)))
}

// start starts o's hooks in a goroutine; the channel delivers the stop or the error.
func (s *lanesSuite) start(ctx context.Context, o *object, changed func()) <-chan started {
	done := make(chan started, 1)

	go func() {
		stop, err := s.router.Start(ctx, context.Background(), o, changed)
		done <- started{stop: stop, err: err}
	}()

	return done
}

type started struct {
	stop func()
	err  error
}

// mustStart starts o and returns its stop, stopped at the test's end.
func (s *lanesSuite) mustStart(o *object) func() {
	res := s.receive(s.start(context.Background(), o, func() {}))
	s.Require().NoError(res.err)
	s.T().Cleanup(res.stop)

	return res.stop
}

func (s *lanesSuite) receive(ch <-chan started) started {
	select {
	case res := <-ch:
		return res
	case <-time.After(waitTimeout):
		s.FailNow("Start did not return")
		return started{}
	}
}

// blocker is a hook program that blocks until released, ignoring ctx when stubborn.
type blocker struct {
	release chan struct{}
}

func newBlocker() *blocker { return &blocker{release: make(chan struct{})} }

func (b *blocker) wait(ctx context.Context, stubborn bool) error {
	if stubborn {
		<-b.release
		return nil
	}

	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *lanesSuite) index(entry string) int {
	return slices.Index(s.calls.list(), entry)
}

// Start waits for the synchronization of a binding that waits, and not for one that does not; both
// unlock their monitors after their run.
func (s *lanesSuite) TestStartWaitsOnlyForTheWaitingSynchronization() {
	block := newBlocker()
	o := &object{name: "app", calls: s.calls, queues: []string{"main", "other"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "waits", binding{queue: "main", wait: true}),
			kubeHook(s.calls, "lazy", binding{queue: "other"}),
		},
		run: func(ctx context.Context, hook string, _ []bctx.BindingContext) error {
			if hook == "waits" {
				return block.wait(ctx, false)
			}

			return nil
		}}

	done := s.start(context.Background(), o, func() {})

	s.Require().True(s.calls.waitFor("run:waits:sync"))
	s.Require().True(s.calls.waitFor("unlock:lazy:lazy-0"), "the lazy binding synchronizes beside the waiting one")
	s.Empty(done, "Start must not return while a waiting binding is not synchronized")

	close(block.release)

	res := s.receive(done)
	s.Require().NoError(res.err)
	s.T().Cleanup(res.stop)

	s.Less(s.index("run:waits:sync"), s.index("unlock:waits:waits-0"), "unlock after the run")
	s.Less(s.index("initialize"), s.index("add-monitor:waits-0"))
}

// D2: synchronizations go into their hook's own lane, the waiting ones first.
func (s *lanesSuite) TestWaitingSynchronizationRunsFirstInItsLane() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "lazy", binding{queue: "main"}),
			kubeHook(s.calls, "waits", binding{queue: "main", wait: true}),
		}}

	s.mustStart(o)
	s.Require().True(s.calls.waitFor("run:lazy:sync"))

	s.Less(s.index("run:waits:sync"), s.index("run:lazy:sync"))
}

// A binding that does not execute on synchronization is only unlocked.
func (s *lanesSuite) TestSynchronizationWithoutExecutionOnlyUnlocks() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", wait: true, noSync: true})}}

	s.mustStart(o)

	s.Contains(s.calls.list(), "unlock:h:h-0")
	s.NotContains(s.calls.list(), "run:h:sync")
}

// The wait is bounded by ctx; Start then stops what it started and returns no stop.
func (s *lanesSuite) TestStartEndsWithCtx() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", wait: true})},
		run: func(ctx context.Context, _ string, _ []bctx.BindingContext) error {
			<-ctx.Done()
			return ctx.Err()
		}}

	ctx, cancel := context.WithCancel(context.Background())
	done := s.start(ctx, o, func() {})

	s.Require().True(s.calls.waitFor("run:h:sync"))
	cancel()

	res := s.receive(done)
	s.Require().ErrorIs(res.err, context.Canceled)
	s.Nil(res.stop)
	s.True(s.calls.waitFor("disable-hooks"))
	s.NotContains(s.calls.list(), "record-failure:h:"+observed.HookReasonSync, "a sync ended by the stop is no failure")
}

// An Enable of a binding that fails stops what it started.
func (s *lanesSuite) TestStartFailingMidwayLeavesNothing() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "h1", binding{queue: "main"}),
			kubeHookOver(&kubeEvents{calls: s.calls, fail: true}, "h2", binding{queue: "main"}),
		}}

	res := s.receive(s.start(context.Background(), o, func() {}))

	s.Require().ErrorContains(res.err, "enable kubernetes bindings of hook 'h2'")
	s.Nil(res.stop)
	s.True(s.calls.waitFor("reset-failures:app"))
	s.Less(s.index("disable-hooks"), s.index("reset-failures:app"))
	s.Empty(s.router.Dump("app").Lanes, "unregistered")
}

// Events of one lane run one at a time in delivery order: coalesced or not, the contexts of the
// runs, in run order, are the events in delivery order, each once.
func (s *lanesSuite) TestEventsRunInDeliveryOrder() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", noSync: true})}}
	s.mustStart(o)

	for _, id := range []string{"e1", "e2", "e3"} {
		s.router.RouteKubeEvent(context.Background(), event("h-0", id))
	}

	var runs []string

	s.Require().True(s.calls.wait(func(e []string) bool {
		runs = runs[:0]
		for _, entry := range e {
			if r, ok := strings.CutPrefix(entry, "run:h:"); ok {
				runs = append(runs, r)
			}
		}

		return strings.Join(runs, ",") == "e1,e2,e3"
	}), "runs: %v", s.calls.list())

	s.True(s.calls.wait(func(e []string) bool { return countOf(e, "resume:app") == len(runs) }))
	s.Equal(len(runs), s.calls.count("pause:app"), "the drift monitor is paused around every run")
}

// Lanes run in parallel: a hook blocked in one lane delays nothing in another.
func (s *lanesSuite) TestLanesRunInParallel() {
	block := newBlocker()
	o := &object{name: "app", calls: s.calls, queues: []string{"a", "b"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "slow", binding{queue: "a", noSync: true}),
			kubeHook(s.calls, "fast", binding{queue: "b", noSync: true}),
		},
		run: func(ctx context.Context, hook string, _ []bctx.BindingContext) error {
			if hook == "slow" {
				return block.wait(ctx, false)
			}

			return nil
		}}
	s.mustStart(o)
	defer close(block.release)

	s.router.RouteKubeEvent(context.Background(), event("slow-0", "e1"))
	s.Require().True(s.calls.waitFor("run:slow:e1"))

	s.router.RouteKubeEvent(context.Background(), event("fast-0", "e2"))
	s.True(s.calls.waitFor("run:fast:e2"))
}

// A failing head blocks its lane only, is recorded, and is retried after its backoff (D3: 5 s,
// doubling, on the engine clock); its success clears the failure, then the item behind it runs.
func (s *lanesSuite) TestFailingHeadBlocksItsLaneAndIsRetried() {
	failures := 0
	o := &object{name: "app", calls: s.calls, queues: []string{"a", "b"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "flaky", binding{queue: "a", noSync: true}),
			kubeHook(s.calls, "other", binding{queue: "a", noSync: true}),
			kubeHook(s.calls, "elsewhere", binding{queue: "b", noSync: true}),
		},
		run: func(_ context.Context, hook string, _ []bctx.BindingContext) error {
			if hook == "flaky" && failures < 2 {
				failures++
				return errors.New("boom")
			}

			return nil
		}}
	s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("flaky-0", "e1"))
	s.router.RouteKubeEvent(context.Background(), event("other-0", "e2"))
	s.Require().True(s.calls.waitFor("record-failure:flaky:" + observed.HookReasonEvent))

	s.router.RouteKubeEvent(context.Background(), event("elsewhere-0", "e3"))
	s.Require().True(s.calls.waitFor("run:elsewhere:e3"), "another lane is not delayed")
	s.NotContains(s.calls.list(), "run:other:e2", "the item behind the failed head waits")

	s.Equal(5*time.Second, s.armedRetry(0))

	d := s.router.Dump("app").Lanes["app"]["a"]
	s.Equal(hooklane.StateRetrying, d.State)
	s.Require().NotNil(d.Failure)
	s.Equal("flaky", d.Failure.Hook)
	s.Equal(1, d.Failure.Attempts)
	s.Equal("5s", d.Failure.NextRetry)

	s.clock.Advance(5 * time.Second)

	// A wake left from e2's delivery may rearm the first delay late; past the second failure's
	// record every arm is the doubled one.
	failed := "record-failure:flaky:" + observed.HookReasonEvent
	s.Require().True(s.calls.wait(func(e []string) bool { return countOf(e, failed) == 2 }))
	mark := slices.Index(s.calls.list(), failed) + 1
	mark += slices.Index(s.calls.list()[mark:], failed) + 1
	s.Equal(10*time.Second, s.armedRetry(mark), "the delay doubles while the same head keeps failing")

	s.clock.Advance(10 * time.Second)

	s.Require().True(s.calls.waitFor("run:other:e2"))
	s.Equal(3, s.calls.count("run:flaky:e1"))
	s.Less(s.index("clear-failure:flaky"), s.index("run:other:e2"))
}

// armedRetry waits for the retry timer a failed head arms after the log's entry after, and returns
// its delay.
func (s *lanesSuite) armedRetry(after int) time.Duration {
	var retryIn time.Duration

	s.Require().True(s.calls.wait(func(e []string) bool {
		for _, entry := range e[min(after, len(e)):] {
			if d, ok := strings.CutPrefix(entry, "armed:"); ok {
				if v, err := time.ParseDuration(d); err == nil && v >= time.Second && v < time.Minute {
					retryIn = v
					return true
				}
			}
		}

		return false
	}), "no retry armed: %v", s.calls.list())

	return retryIn
}

// D4: a failure the item allows is not recorded as the package's failure and does not block the
// lane: the item leaves it, and the lane counts it.
func (s *lanesSuite) TestAllowedFailureLeavesTheLane() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "tolerant", binding{queue: "main", noSync: true, allowErr: true}),
			kubeHook(s.calls, "next", binding{queue: "main", noSync: true}),
		},
		run: func(_ context.Context, hook string, _ []bctx.BindingContext) error {
			if hook == "tolerant" {
				return errors.New("boom")
			}

			return nil
		}}
	s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("tolerant-0", "e1"))
	s.router.RouteKubeEvent(context.Background(), event("next-0", "e2"))

	s.Require().True(s.calls.waitFor("run:next:e2"))
	s.Equal(1, s.calls.count("run:tolerant:e1"), "not retried")
	s.NotContains(s.calls.list(), "record-failure:tolerant:"+observed.HookReasonEvent)

	d := s.router.Dump("app").Lanes["app"]["main"]
	s.Equal(1, d.Stats.AllowedFailures)
	s.Contains(d.Stats.LastAllowedFailure, "tolerant: ")
	s.Nil(d.Failure)
}

// D4: a synchronization that fails but allows it unlocks its monitor and lets Start return.
func (s *lanesSuite) TestAllowedSynchronizationFailureUnlocks() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", wait: true, allowErr: true})},
		run:  func(context.Context, string, []bctx.BindingContext) error { return errors.New("boom") }}

	s.mustStart(o)

	s.Contains(s.calls.list(), "unlock:h:h-0")
}

// A stop while the head waits for its retry returns at once, without the retry.
func (s *lanesSuite) TestStopDuringRetryReturns() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", noSync: true})},
		run:  func(context.Context, string, []bctx.BindingContext) error { return errors.New("boom") }}
	stop := s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e1"))
	s.armedRetry(0)

	stop()

	s.Equal(1, s.calls.count("run:h:e1"))
	s.Contains(s.calls.list(), "disable-hooks")
}

// D1: the stop waits for the run in flight without a bound, warns while it waits, and disables the
// hooks only after the run returned.
func (s *lanesSuite) TestStopWaitsForTheRunInFlight() {
	block := newBlocker()
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "stuck", binding{queue: "main", noSync: true})},
		run: func(ctx context.Context, _ string, _ []bctx.BindingContext) error {
			err := block.wait(ctx, true)
			s.calls.add("returned")

			return err
		}}
	stop := s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("stuck-0", "e1"))
	s.Require().True(s.calls.waitFor("run:stuck:e1"))

	stopped := make(chan struct{})

	go func() {
		stop()
		close(stopped)
	}()

	s.Require().True(s.calls.waitFor("armed:10s"), "the stop's warning timer")
	s.clock.Advance(10 * time.Second)
	s.Require().True(s.calls.wait(func([]string) bool { return strings.Contains(s.logs.String(), "stop waits for a hook in flight") }))
	s.Contains(s.logs.String(), "stuck")

	s.Empty(stopped, "the stop waits for the hook")
	s.NotContains(s.calls.list(), "disable-hooks", "no controller is cleared under a running hook")

	close(block.release)
	<-stopped

	s.Less(s.index("returned"), s.index("disable-hooks"))
}

// PF2 by construction: an event delivered after the stop began is dropped; no lane is recreated.
func (s *lanesSuite) TestDeliveryAfterStopIsDropped() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", noSync: true})}}
	stop := s.mustStart(o)

	stop()
	s.router.RouteKubeEvent(context.Background(), event("h-0", "late"))

	s.NotContains(s.calls.list(), "run:h:late")
	s.Empty(s.router.Dump("").Lanes)
}

// A dump while a hook runs returns at once and shows the run in flight and what waits.
func (s *lanesSuite) TestDumpDuringARun() {
	block := newBlocker()
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", noSync: true})},
		run: func(ctx context.Context, _ string, contexts []bctx.BindingContext) error {
			if ids(contexts) == "e1" {
				return block.wait(ctx, false)
			}

			return nil
		}}
	s.mustStart(o)
	defer close(block.release)

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e1"))
	s.Require().True(s.calls.waitFor("run:h:e1"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e2"))

	d := s.router.Dump("app").Lanes["app"]["main"]

	s.Equal(hooklane.StateRunning, d.State)
	s.Require().NotNil(d.InFlight)
	s.Equal("h", d.InFlight.Hook)
	s.Equal(hooklane.KindEvent, d.InFlight.Kind)
	s.Equal(1, d.Waiting.Count)
	s.Equal(map[string]int{"h": 1}, d.Waiting.ByHook)

	summary := s.router.Summary("app")
	s.Require().Len(summary, 1)
	s.Equal(hooklane.StateRunning, summary[0].State)
}

// Every run that changes the values checksum calls changed: a synchronization too.
func (s *lanesSuite) TestSynchronizationThatPatchesValuesCallsChanged() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", wait: true})}}
	o.run = func(context.Context, string, []bctx.BindingContext) error {
		o.setChecksum("patched")
		return nil
	}

	changed := 0
	res := s.receive(s.start(context.Background(), o, func() { changed++ }))
	s.Require().NoError(res.err)
	s.T().Cleanup(res.stop)

	s.Equal(1, changed)
}

// A schedule tick runs its hook in its lane.
func (s *lanesSuite) TestScheduleTickRuns() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		schedule: []hooks.ControllableHook{scheduleHook("tick", "main", "* * * * *")}}
	s.mustStart(o)

	s.router.RouteSchedule(context.Background(), "* * * * *")

	s.True(s.calls.waitFor("run:tick:schedule"))
}

// Serve delivers what the managers' channels carry, kube events and schedule ticks, and returns
// once done is closed.
func (s *lanesSuite) TestServeRoutesUntilDone() {
	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube:     []hooks.ControllableHook{kubeHook(s.calls, "h", binding{queue: "main", noSync: true})},
		schedule: []hooks.ControllableHook{scheduleHook("tick", "main", "* * * * *")}}
	s.mustStart(o)

	kube := make(chan kemtypes.KubeEvent)
	schedule := make(chan string)
	done := make(chan struct{})
	exited := make(chan struct{})

	go func() {
		defer close(exited)
		s.router.Serve(done, kube, schedule)
	}()

	kube <- event("h-0", "e1")
	s.Require().True(s.calls.waitFor("run:h:e1"))

	schedule <- "* * * * *"
	s.Require().True(s.calls.waitFor("run:tick:schedule"))

	close(done)
	<-exited
}

// An update or a restart starts new lanes under the same name; the old stop, if it comes after the
// new start, removes only its own registration.
func (s *lanesSuite) TestNewLanesSurviveTheOldStop() {
	old := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "old", binding{queue: "main", noSync: true})}}
	stopOld := s.mustStart(old)

	o := &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{kubeHook(s.calls, "new", binding{queue: "main", noSync: true})}}
	s.mustStart(o)

	stopOld()
	s.router.RouteKubeEvent(context.Background(), event("new-0", "e1"))

	s.True(s.calls.waitFor("run:new:e1"))
	s.Len(s.router.Dump("app").Lanes, 1)
}

// countOf counts entry in e (a wait condition must not take the log's lock again).
func countOf(e []string, entry string) int {
	n := 0

	for _, x := range e {
		if x == entry {
			n++
		}
	}

	return n
}

// coalesceObject is a package with hook "h" on monitor "h-0" (in group, when set) and hook "b" on
// "b-0" in one lane; a run whose first context is block's id blocks until released.
func (s *lanesSuite) coalesceObject(group, block string, release <-chan struct{}) *object {
	return &object{name: "app", calls: s.calls, queues: []string{"main"},
		kube: []hooks.ControllableHook{
			kubeHook(s.calls, "h", binding{queue: "main", noSync: true, group: group}),
			kubeHook(s.calls, "b", binding{queue: "main", noSync: true}),
		},
		run: func(ctx context.Context, _ string, contexts []bctx.BindingContext) error {
			if len(contexts) > 0 && strings.HasPrefix(ids(contexts), block) {
				select {
				case <-release:
				case <-ctx.Done():
				}
			}

			return nil
		}}
}

// tenWhileRunning delivers e1..e10 for h while a run of e0 holds the lane, checks that they wait
// as one item of contexts binding contexts, then releases the lane.
func (s *lanesSuite) tenWhileRunning(release chan struct{}, contexts int) {
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e0"))
	s.Require().True(s.calls.waitFor("run:h:e0"))

	for i := 1; i <= 10; i++ {
		s.router.RouteKubeEvent(context.Background(), event("h-0", "e"+strconv.Itoa(i)))
	}

	d := s.router.Dump("app").Lanes["app"]["main"]
	s.Equal(1, d.Waiting.Count, "ten events of one hook wait as one item")
	s.Equal(contexts, d.Waiting.Items[0].Contexts)

	close(release)
}

// Coalescing: ten events for an ungrouped hook that arrive while it runs run it once with the ten
// contexts in order.
func (s *lanesSuite) TestTenEventsRunOnceWithTenContexts() {
	release := make(chan struct{})
	s.mustStart(s.coalesceObject("", "e0", release))

	s.tenWhileRunning(release, 10)

	s.Require().True(s.calls.waitFor("run:h:e1,e2,e3,e4,e5,e6,e7,e8,e9,e10"))
	s.Equal(2, countOf(s.calls.list(), "pause:app"), "two runs")
}

// Coalescing: a grouped hook reads snapshots, so of ten contexts of its group only the last is
// kept, already while they wait (shell-operator's group compaction, done at delivery).
func (s *lanesSuite) TestTenEventsOfAGroupRunOnceWithTheLast() {
	release := make(chan struct{})
	s.mustStart(s.coalesceObject("main", "e0", release))

	s.tenWhileRunning(release, 1)

	s.True(s.calls.waitFor("run:h:e10"))
}

// Coalescing never reorders hooks: an event of another hook between two of h's keeps them apart.
func (s *lanesSuite) TestCoalescingKeepsTheOrderBetweenHooks() {
	release := make(chan struct{})
	s.mustStart(s.coalesceObject("", "e0", release))

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e0"))
	s.Require().True(s.calls.waitFor("run:h:e0"))

	s.router.RouteKubeEvent(context.Background(), event("h-0", "a1"))
	s.router.RouteKubeEvent(context.Background(), event("b-0", "b1"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "a2"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "a3"))
	close(release)

	s.Require().True(s.calls.waitFor("run:h:a2,a3"))
	s.Less(s.index("run:h:a1"), s.index("run:b:b1"))
	s.Less(s.index("run:b:b1"), s.index("run:h:a2,a3"))
}

// Coalescing: a failed batch is retried with the batch plus what arrived while it waited.
func (s *lanesSuite) TestFailedBatchIsRetriedWithTheNewcomers() {
	release := make(chan struct{})
	failed := false
	o := s.coalesceObject("", "e0", release)
	inner := o.run
	o.run = func(ctx context.Context, hook string, contexts []bctx.BindingContext) error {
		if ids(contexts) == "e1,e2" && !failed {
			failed = true
			return errors.New("boom")
		}

		return inner(ctx, hook, contexts)
	}
	s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e0"))
	s.Require().True(s.calls.waitFor("run:h:e0"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e1"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e2"))
	close(release)

	s.Require().True(s.calls.waitFor("run:h:e1,e2"))
	s.armedRetry(0)

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e3"))
	s.clock.Advance(5 * time.Second)

	s.True(s.calls.waitFor("run:h:e1,e2,e3"))
}

// D4: a hook with executionMinInterval runs at its interval with everything that arrived meanwhile.
func (s *lanesSuite) TestRateLimitedHookRunsAtItsIntervalWithEverything() {
	limited := kubeHook(s.calls, "h", binding{queue: "main", noSync: true})
	limited.settings = &shtypes.Settings{ExecutionMinInterval: time.Minute, ExecutionBurst: 1}

	o := &object{name: "app", calls: s.calls, queues: []string{"main"}, kube: []hooks.ControllableHook{limited}}
	s.mustStart(o)

	s.router.RouteKubeEvent(context.Background(), event("h-0", "e1"))
	s.Require().True(s.calls.waitFor("run:h:e1"), "the first run uses the burst: %v", s.calls.list())

	mark := len(s.calls.list())
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e2"))
	s.router.RouteKubeEvent(context.Background(), event("h-0", "e3"))

	s.Require().True(s.calls.wait(func(e []string) bool { return slices.Contains(e[mark:], "armed:1m0s") }))
	s.Equal(hooklane.StateThrottled, s.router.Dump("app").Lanes["app"]["main"].State)
	s.NotContains(s.calls.list(), "run:h:e2")

	s.clock.Advance(time.Minute)

	s.True(s.calls.waitFor("run:h:e2,e3"))
}
