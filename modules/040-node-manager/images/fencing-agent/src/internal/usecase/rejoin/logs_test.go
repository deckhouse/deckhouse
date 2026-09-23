/*
Copyright 2026 Flant JSC

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

package rejoin

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/logtest"
)

const (
	startedMsg         = "gossip quorum lost, rejoin started"
	startedByRecordMsg = "a peer recorded this node as failed, rejoin started although gossip quorum holds"
	triggerChangedMsg  = "rejoin trigger changed"
	finishedMsg        = "rejoin finished, gossip quorum holds and no peer records this node as failed"
	failedMsg          = "rejoin attempt failed"
	streakEndedMsg     = "rejoin failure streak ended"
	joinedMsg          = "rejoin attempt joined, the trigger remains"
	stoppedMsg         = "rejoin stopped by shutdown"
)

func indexOfMsg(records []logtest.Record, msg string, n int) int {
	seen := 0

	for i, record := range records {
		if record.Msg() != msg {
			continue
		}

		seen++
		if seen == n {
			return i
		}
	}

	return -1
}

func infoOrAbove(records []logtest.Record) []logtest.Record {
	var kept []logtest.Record

	for _, record := range records {
		switch record.Level() {
		case "info", "warn", "error", "fatal":
			kept = append(kept, record)
		}
	}

	return kept
}

func messages(records []logtest.Record) []string {
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.Msg())
	}

	return got
}

func assertDuration(t *testing.T, record logtest.Record, key string) {
	t.Helper()

	if _, err := time.ParseDuration(record.Str(key)); err != nil {
		t.Errorf("%q %s = %v, want a duration: %v", record.Msg(), key, record[key], err)
	}
}

func TestRejoinFailureStreakWarnsOnceThenDebugs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		h := newLoggedHarness(t, testParams, logtest.NewJSONLogger(logs))
		h.failAttempts(errTransport)
		h.quorumAfter(5)

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return strings.Contains(logs.String(), finishedMsg) })

		h.setQuorum(false)

		waitFor(t, func() bool { return h.attempts.Load() == 6 })

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		failed := logtest.WithMsg(records, failedMsg)
		if len(failed) != 6 {
			t.Fatalf("%q records = %v, want 6", failedMsg, failed)
		}

		if got, want := logtest.Levels(failed[:5]), []string{"warn", "debug", "debug", "debug", "debug"}; !slices.Equal(got, want) {
			t.Errorf("%q levels in the first episode = %v, want %v", failedMsg, got, want)
		}

		if got := failed[5].Level(); got != "warn" {
			t.Errorf("%q level of the first failure in the next episode = %q, want warn", failedMsg, got)
		}

		finished := logtest.WithMsg(records, finishedMsg)
		if len(finished) != 1 || finished[0].Level() != "info" {
			t.Errorf("%q records = %v, want one at info", finishedMsg, finished)
		}

		if got := len(logtest.WithMsg(records, streakEndedMsg)); got != 0 {
			t.Errorf("%q logged %d times, want 0: the episode summary closes the streak", streakEndedMsg, got)
		}

		lastOfFirst, end, firstOfNext := indexOfMsg(records, failedMsg, 5), indexOfMsg(records, finishedMsg, 1), indexOfMsg(records, failedMsg, 6)
		if lastOfFirst >= end || end >= firstOfNext {
			t.Errorf("log positions: fifth failure %d, finish %d, sixth failure %d; want the finish between them", lastOfFirst, end, firstOfNext)
		}
	})
}

func TestStreakStartWarnCarriesTheAttemptElapsed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const attemptDuration = 1500 * time.Millisecond

		logs := &syncBuffer{}
		h := newLoggedHarness(t, testParams, logtest.NewJSONLogger(logs))
		h.setAttemptResult(func(ctx context.Context, _ int64) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(attemptDuration):
				return errTransport
			}
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return strings.Contains(logs.String(), failedMsg) })

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		failed := logtest.WithMsg(records, failedMsg)
		if len(failed) != 1 || failed[0].Level() != "warn" {
			t.Fatalf("%q records = %v, want one at warn", failedMsg, failed)
		}

		if got := failed[0].Str("attempt_elapsed"); got != "1.5s" {
			t.Errorf("%q attempt_elapsed = %q, want \"1.5s\"", failedMsg, got)
		}

		if got := failed[0].Int("attempt"); got != 1 {
			t.Errorf("%q attempt = %d, want 1", failedMsg, got)
		}

		nextIn, err := time.ParseDuration(failed[0].Str("next_in"))
		if err != nil {
			t.Fatalf("%q next_in = %v, want a duration: %v", failedMsg, failed[0]["next_in"], err)
		}

		within(t, nextIn, interval)
	})
}

func TestEpisodeEndLogsOneSummary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		h := newLoggedHarness(t, testParams, logtest.NewJSONLogger(logs))
		h.setAttemptResult(func(_ context.Context, n int64) error {
			if n <= 3 {
				return errTransport
			}

			h.setQuorum(true)

			return nil
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return strings.Contains(logs.String(), finishedMsg) })

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		sleeps := h.recordedSleeps()
		if len(sleeps) != 3 {
			t.Fatalf("sleeps = %v, want one after each of the 3 failed attempts", sleeps)
		}

		finished := logtest.WithMsg(records, finishedMsg)
		if len(finished) != 1 {
			t.Fatalf("%q records = %v, want exactly one", finishedMsg, finished)
		}

		summary := finished[0]

		if got := summary.Level(); got != "info" {
			t.Errorf("%q level = %q, want info", finishedMsg, got)
		}

		if got := summary.Int("attempts"); got != 4 {
			t.Errorf("%q attempts = %d, want 4", finishedMsg, got)
		}

		if got, want := summary.Str("elapsed"), (sleeps[0] + sleeps[1] + sleeps[2]).Truncate(time.Millisecond).String(); got != want {
			t.Errorf("%q elapsed = %q, want %q", finishedMsg, got, want)
		}

		if got, want := summary.Str("last_delay"), sleeps[2].String(); got != want {
			t.Errorf("%q last_delay = %q, want %q, the last sleep", finishedMsg, got, want)
		}

		if got := summary.Str("last_error_class"); got != classTransport {
			t.Errorf("%q last_error_class = %q, want %q", finishedMsg, got, classTransport)
		}

		if got := len(logtest.WithMsg(records, streakEndedMsg)); got != 0 {
			t.Errorf("%q logged %d times, want 0: the episode summary closes the streak", streakEndedMsg, got)
		}
	})
}

func TestStreakEndWithoutEpisodeEndLogsASummary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		h := newLoggedHarness(t, testParams, logtest.NewJSONLogger(logs))
		h.setQuorum(true)
		h.setOwn(true)
		h.setAttemptResult(func(_ context.Context, n int64) error {
			switch {
			case n <= 2:
				return errTransport
			case n <= 4:
				return errNotMember
			default:
				return nil
			}
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() == 5 })

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		if got := len(logtest.WithMsg(records, finishedMsg)); got != 0 {
			t.Errorf("%q logged %d times while the failed record remains, want 0", finishedMsg, got)
		}

		failed := logtest.WithMsg(records, failedMsg)
		if got, want := logtest.Levels(failed), []string{"warn", "debug"}; !slices.Equal(got, want) {
			t.Errorf("%q levels = %v, want %v", failedMsg, got, want)
		}

		notMember := logtest.WithMsg(records, notMemberVerdict)
		if got, want := logtest.Levels(notMember), []string{"warn", "debug"}; !slices.Equal(got, want) {
			t.Errorf("%q levels = %v, want %v", notMemberVerdict, got, want)
		}

		for _, record := range append(slices.Clone(failed), notMember...) {
			if got := record.Str("attempt_elapsed"); got != "0s" {
				t.Errorf("%q attempt %d attempt_elapsed = %q, want \"0s\"", record.Msg(), record.Int("attempt"), got)
			}
		}

		ended := logtest.WithMsg(records, streakEndedMsg)
		if len(ended) != 2 {
			t.Fatalf("%q records = %v, want one at the class change and one at the successful attempt", streakEndedMsg, ended)
		}

		sleeps := h.recordedSleeps()
		if len(sleeps) < 4 {
			t.Fatalf("sleeps = %v, want at least 4", sleeps)
		}

		for i, want := range []struct {
			class     string
			lastDelay time.Duration
		}{
			{class: classTransport, lastDelay: sleeps[1]},
			{class: classNotMember, lastDelay: sleeps[3]},
		} {
			summary := ended[i]

			if got := summary.Level(); got != "info" {
				t.Errorf("%q #%d level = %q, want info", streakEndedMsg, i+1, got)
			}

			if got := summary.Int("attempts"); got != 2 {
				t.Errorf("%q #%d attempts = %d, want 2", streakEndedMsg, i+1, got)
			}

			if got := summary.Str("last_error_class"); got != want.class {
				t.Errorf("%q #%d last_error_class = %q, want %q", streakEndedMsg, i+1, got, want.class)
			}

			if got := summary.Str("last_delay"); got != want.lastDelay.String() {
				t.Errorf("%q #%d last_delay = %q, want %q", streakEndedMsg, i+1, got, want.lastDelay)
			}

			assertDuration(t, summary, "elapsed")
		}

		transportStart, classChange, notMemberStart := indexOfMsg(records, failedMsg, 1), indexOfMsg(records, streakEndedMsg, 1), indexOfMsg(records, notMemberVerdict, 1)
		if transportStart >= classChange || classChange >= notMemberStart {
			t.Errorf("log positions: transport warn %d, streak end %d, not-member warn %d; want the streak end between the warns", transportStart, classChange, notMemberStart)
		}

		lastNotMember, successEnd := indexOfMsg(records, notMemberVerdict, 2), indexOfMsg(records, streakEndedMsg, 2)
		if lastNotMember >= successEnd {
			t.Errorf("log positions: last not-member failure %d, streak end %d; want the streak end after it", lastNotMember, successEnd)
		}
	})
}

func cutOwnReadShort(h *harness, read int64) {
	h.setOnOwnRead(func(ctx context.Context, n int64) bool {
		if n < read {
			return true
		}

		select {
		case <-ctx.Done():
		case <-h.ctx.Done():
		}

		if ctx.Err() == nil {
			h.t.Errorf("OwnFailedRecord read %d got a ctx that shutdown does not cancel", n)
		}

		return false
	})
}

func TestRejoinStopsOnCancelDuringSleepAndDuringAnAttempt(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(h *harness)
		reach    func(t *testing.T, h *harness)
		warns    []string
		open     bool
		attempts int64
		sleeps   int
	}{
		{
			name:  "cancel during a sleep",
			setup: func(*harness) {},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return len(h.recordedSleeps()) == 2 })
			},
			open:     true,
			attempts: 2,
			sleeps:   2,
		},
		{
			name: "cancel while an attempt waits for ctx",
			setup: func(h *harness) {
				h.setAttemptResult(func(ctx context.Context, n int64) error {
					if n == 1 {
						return nil
					}

					<-ctx.Done()

					return ctx.Err()
				})
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.attempts.Load() == 2 })
			},
			open:     true,
			attempts: 2,
			sleeps:   1,
		},
		{
			name: "cancel during the trigger read before an attempt",
			setup: func(h *harness) {
				h.setQuorum(true)
				cutOwnReadShort(h, 3)
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.ownReads.Load() == 3 })
			},
			warns:    []string{startedByRecordMsg},
			open:     true,
			attempts: 1,
			sleeps:   1,
		},
		{
			name: "cancel during the trigger read after a successful attempt",
			setup: func(h *harness) {
				h.setQuorum(true)
				cutOwnReadShort(h, 4)
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.ownReads.Load() == 4 })
			},
			warns:    []string{startedByRecordMsg},
			open:     true,
			attempts: 2,
			sleeps:   1,
		},
		{
			name:  "cancel while idle without an episode",
			setup: func(h *harness) { h.setQuorum(true) },
			reach: func(*testing.T, *harness) { idleFor(3 * idleTick) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				logs := &syncBuffer{}
				h := newLoggedHarness(t, testParams, logtest.NewJSONLogger(logs))
				tc.setup(h)

				start := time.Now()
				done := make(chan struct{})

				var runErr error

				go func() {
					defer close(done)

					runErr = h.loop.Run(h.ctx)
				}()

				tc.reach(t, h)

				elapsed := time.Since(start)
				h.cancel()

				synctest.Wait()

				select {
				case <-done:
				default:
					t.Fatal("Run did not return right after cancel")
				}

				if runErr != nil {
					t.Errorf("Run = %v on shutdown, want nil", runErr)
				}

				if got := h.attempts.Load(); got != tc.attempts {
					t.Errorf("attempts = %d, want %d", got, tc.attempts)
				}

				sleeps := h.recordedSleeps()
				if len(sleeps) != tc.sleeps {
					t.Fatalf("sleeps = %v, want %d", sleeps, tc.sleeps)
				}

				records := logtest.Decode(t, logs.String())
				logtest.AssertSnakeCaseKeys(t, records)

				var warned []string

				for _, record := range records {
					if record.Level() == "warn" {
						warned = append(warned, record.Msg())
					}
				}

				if !slices.Equal(warned, tc.warns) {
					t.Errorf("warn messages = %v, want %v: stopping the loop warns nothing", warned, tc.warns)
				}

				if got := len(logtest.WithMsg(records, finishedMsg)); got != 0 {
					t.Errorf("%q logged %d times on shutdown, want 0", finishedMsg, got)
				}

				stopped := logtest.WithMsg(records, stoppedMsg)

				if !tc.open {
					if len(stopped) != 0 {
						t.Errorf("%q records = %v with no episode open, want none", stoppedMsg, stopped)
					}

					return
				}

				if len(stopped) != 1 {
					t.Fatalf("%q records = %v, want exactly one", stoppedMsg, stopped)
				}

				summary := stopped[0]

				if got := summary.Level(); got != "info" {
					t.Errorf("%q level = %q, want info", stoppedMsg, got)
				}

				if got := summary.Int("attempts"); int64(got) != tc.attempts {
					t.Errorf("%q attempts = %d, want %d", stoppedMsg, got, tc.attempts)
				}

				if got, want := summary.Str("elapsed"), elapsed.Truncate(time.Millisecond).String(); got != want {
					t.Errorf("%q elapsed = %q, want %q", stoppedMsg, got, want)
				}

				if got, want := summary.Str("last_delay"), sleeps[len(sleeps)-1].String(); got != want {
					t.Errorf("%q last_delay = %q, want %q", stoppedMsg, got, want)
				}

				if got := summary.Str("last_error_class"); got != classNone {
					t.Errorf("%q last_error_class = %q, want %q", stoppedMsg, got, classNone)
				}

				if got := indexOfMsg(records, stoppedMsg, 1); got != len(records)-1 {
					t.Errorf("%q is record %d of %d, want the last one", stoppedMsg, got, len(records))
				}
			})
		})
	}
}

func TestOwnRecordEpisodeLogsNothingPerAttemptAtInfo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const episodeAttempts = 20

		params := Params{Interval: 100 * time.Millisecond, MaxInterval: 400 * time.Millisecond}

		logs := &syncBuffer{}
		h := newLoggedHarness(t, params, logtest.NewJSONLogger(logs))
		h.setQuorum(true)
		h.setOwn(true)
		h.setAttemptResult(func(_ context.Context, n int64) error {
			if n == episodeAttempts {
				h.setOwn(false)
			}

			return nil
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return strings.Contains(logs.String(), finishedMsg) })
		idleFor(3 * idleTick)

		if got := h.attempts.Load(); got != episodeAttempts {
			t.Fatalf("attempts = %d, want %d", got, episodeAttempts)
		}

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		loud := infoOrAbove(records)
		if got, want := messages(loud), []string{startedByRecordMsg, finishedMsg}; !slices.Equal(got, want) {
			t.Fatalf("records at info or above = %v, want %v and nothing per attempt", got, want)
		}

		if got := loud[0].Level(); got != "warn" {
			t.Errorf("%q level = %q, want warn", startedByRecordMsg, got)
		}

		if got := loud[1].Level(); got != "info" {
			t.Errorf("%q level = %q, want info", finishedMsg, got)
		}

		if got := loud[1].Int("attempts"); got != episodeAttempts {
			t.Errorf("%q attempts = %d, want %d", finishedMsg, got, episodeAttempts)
		}

		joined := logtest.WithMsg(records, joinedMsg)
		if len(joined) != episodeAttempts-1 {
			t.Errorf("%q records = %d, want %d", joinedMsg, len(joined), episodeAttempts-1)
		}

		for _, record := range joined {
			if got := record.Level(); got != "debug" {
				t.Errorf("%q attempt %d level = %q, want debug", joinedMsg, record.Int("attempt"), got)
			}
		}
	})
}
