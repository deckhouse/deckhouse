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

package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Default flag values of the proxy (config.Parse).
const (
	defaultInterval  = time.Second
	defaultTimeout   = 500 * time.Millisecond
	defaultWindow    = 30 * time.Second
	failStepInterval = 500 * time.Millisecond
	defaultJitter    = 0.2
)

func defaultOptions() []ListOption {
	return []ListOption{
		WithHealthcheckInterval(defaultInterval),
		WithHealthcheckTimeout(defaultTimeout),
		WithHealthCheckJitter(defaultJitter),
		WithFailStep(failStepInterval, defaultWindow),
	}
}

func newFakeClockList(t *testing.T, opts []ListOption, addrs ...string) (*List, *fakeProbes, *fakeClock) {
	t.Helper()

	probes := newFakeProbes()
	for _, addr := range addrs {
		probes.set(addr, probeReady, time.Millisecond)
	}

	list, err := newList(nil, byAddr, append(opts, withProbes(probes))...)
	if err != nil {
		t.Fatal(err)
	}

	clock := &fakeClock{now: time.Unix(1000, 0)}
	list.now = clock.Now
	list.Reconcile(ups(addrs...))

	return list, probes, clock
}

func setAll(probes *fakeProbes, kind probeKind, addrs ...string) {
	for _, addr := range addrs {
		probes.set(addr, kind, time.Millisecond)
	}
}

// assertDelays checks the delay at both jitter limits of the base interval.
func assertDelays(t *testing.T, list *List, base time.Duration) {
	t.Helper()

	lo := time.Duration((1 - defaultJitter) * float64(base))
	hi := time.Duration((1 + defaultJitter) * float64(base))

	if got := list.cycleDelay(-1); got != lo {
		t.Errorf("delay at -jitter = %v, want %v", got, lo)
	}

	if got := list.cycleDelay(1); got != hi {
		t.Errorf("delay at +jitter = %v, want %v", got, hi)
	}

	for range 100 {
		if got := list.cycleDelay(jitterSpread()); got < lo || got > hi {
			t.Fatalf("delay = %v, want in [%v, %v]", got, lo, hi)
		}
	}
}

// lastReadyAgoMs in /upstreams: the upstream is ready.
func TestExportNodes_LastReadyAgo(t *testing.T) {
	list, probes, clock := newTestList(t, "a")
	round(list)

	clock.Advance(300 * time.Millisecond)

	if n := exported(t, list, "a"); n.LastReadyAgoMs == nil || *n.LastReadyAgoMs != 300 {
		t.Fatalf("lastReadyAgoMs = %v, want 300", n.LastReadyAgoMs)
	}

	// A not-ready answer is not a ready probe: the age keeps growing.
	probes.set("a", probeNotReady, time.Millisecond)
	round(list)
	clock.Advance(200 * time.Millisecond)

	if n := exported(t, list, "a"); n.LastReadyAgoMs == nil || *n.LastReadyAgoMs != 500 {
		t.Errorf("lastReadyAgoMs after not-ready = %v, want 500", n.LastReadyAgoMs)
	}
}

// lastReadyAgoMs in /upstreams: the upstream was never ready.
func TestExportNodes_NeverReadyHasNoLastReadyAgo(t *testing.T) {
	list, probes, _ := newTestList(t, "a", "b")
	probes.set("b", probeHard, 0)
	round(list)

	probes.set("b", probeNotReady, time.Millisecond)
	round(list)

	if n := exported(t, list, "b"); n.LastReadyAgoMs != nil {
		t.Fatalf("lastReadyAgoMs = %d, want no field", *n.LastReadyAgoMs)
	}

	raw, err := json.Marshal(exported(t, list, "b"))
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}

	if _, ok := fields["lastReadyAgoMs"]; ok {
		t.Errorf("JSON %s has lastReadyAgoMs, want no field", raw)
	}
}

// lastReadyAgoMs in /upstreams: the old fields stay for an old consumer.
func TestExportNodes_OldFieldsKept(t *testing.T) {
	list, _, _ := newTestList(t, "a")
	round(list)

	raw, err := json.Marshal(exported(t, list, "a"))
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"upstream", "score", "tier", "effectiveTier", "missing", "lastProbe",
		"warm", "downed", "held", "stalling", "evictions", "lastReadyAgoMs",
	}
	if got := slices.Sorted(maps.Keys(fields)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Fatalf("fields = %v, want %v", got, want)
	}

	if fields["upstream"] != "a" || fields["score"] != 2.0 || fields["tier"] != 0.0 || fields["lastProbe"] != "ready" {
		t.Errorf("old fields changed: %s", raw)
	}
}

// gatedProbes answers ready at once for the addresses in ready. For other
// addresses a probe waits until gate is closed and answers timeout.
func gatedProbes(gate <-chan struct{}, ready ...string) func(context.Context, *Upstream) (probeResult, error) {
	return func(_ context.Context, u *Upstream) (probeResult, error) {
		if slices.Contains(ready, u.Addr) {
			return probeResult{kind: probeReady, latency: time.Millisecond}, nil
		}

		<-gate

		return probeResult{kind: probeSoft}, nil
	}
}

// waitReady waits until /upstreams shows a ready answer of the upstream and
// fails the test after 5 s.
func waitReady(t *testing.T, list *List, addr string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for lastReadyAgo(t, list, addr) < 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no ready answer of %s", addr)
		}

		time.Sleep(time.Millisecond)
	}
}

// lastReadyAgo returns lastReadyAgoMs of the upstream, or -1 without the field.
func lastReadyAgo(t *testing.T, list *List, addr string) int64 {
	t.Helper()

	n := exported(t, list, addr)
	if n.LastReadyAgoMs == nil {
		return -1
	}

	return *n.LastReadyAgoMs
}

// lastReadyAgoMs in /upstreams is the time since the ready answer, also when
// another upstream keeps the round open: the ready answer is visible at once,
// and the age is not counted from the end of the round.
func TestExportNodes_LastReadyAgoFromAnswer(t *testing.T) {
	gate := make(chan struct{})

	list, err := newList(nil, byAddr, withProbe(gatedProbes(gate, "a")))
	if err != nil {
		t.Fatal(err)
	}

	clock := &fakeClock{now: time.Unix(0, 0)}
	list.now = clock.Now
	list.Reconcile(ups("a", "b"))

	done := make(chan struct{})
	go func() {
		defer close(done)

		round(list)
	}()

	// "a" answered, "b" keeps the round open.
	waitReady(t, list, "a")
	clock.Advance(1200 * time.Millisecond)

	if got := lastReadyAgo(t, list, "a"); got != 1200 {
		t.Errorf("lastReadyAgoMs before the end of the round = %d, want 1200", got)
	}

	close(gate)
	<-done
	clock.Advance(300 * time.Millisecond)

	if got := lastReadyAgo(t, list, "a"); got != 1500 {
		t.Errorf("lastReadyAgoMs after the end of the round = %d, want 1500", got)
	}
}

// fakeBoot is a settable boot clock in milliseconds.
type fakeBoot struct {
	mu sync.Mutex
	ms uint64
}

func (b *fakeBoot) Millis() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.ms
}

func (b *fakeBoot) Set(ms uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ms = ms
}

func withBoot(b *fakeBoot) ListOption {
	return func(l *ListConfig) error {
		l.boot = b.Millis

		return nil
	}
}

// The number and the result of the last probe, and the monotonic probe number:
// the number is the boot clock at the start of the round.
func TestRound_ProbeSeqAndResult(t *testing.T) {
	list, probes, clock := newTestList(t, "a", "b")
	boot := &fakeBoot{ms: 5000}
	list.boot = boot.Millis

	if seq, ok := list.Probe(); seq != 0 || ok {
		t.Fatalf("before any round Probe() = %d, %v, want 0, false", seq, ok)
	}

	round(list)

	if seq, ok := list.Probe(); seq != 5000 || !ok {
		t.Fatalf("Probe() = %d, %v, want 5000 and true", seq, ok)
	}

	// One ready upstream is enough.
	probes.set("a", probeNotReady, time.Millisecond)
	boot.Set(5500)
	round(list)

	if seq, ok := list.Probe(); seq != 5500 || !ok {
		t.Fatalf("Probe() = %d, %v, want 5500 and true", seq, ok)
	}

	// The wall clock goes back: the number does not change its course.
	clock.Advance(-time.Hour)
	probes.set("b", probeSoft, 0)
	boot.Set(6000)
	round(list)

	if seq, ok := list.Probe(); seq != 6000 || ok {
		t.Fatalf("Probe() = %d, %v, want 6000 and false", seq, ok)
	}
}

// Monotonic probe number: a round that starts in the same
// millisecond as the previous one, or earlier by the clock, still gets a
// larger number.
func TestRound_SeqStrictlyGrows(t *testing.T) {
	list, _, _ := newTestList(t, "a")
	boot := &fakeBoot{ms: 1000}
	list.boot = boot.Millis

	round(list)
	round(list)

	if seq, _ := list.Probe(); seq != 1001 {
		t.Fatalf("Probe() = %d, want 1001 for the same clock value", seq)
	}

	boot.Set(900)
	round(list)

	if seq, _ := list.Probe(); seq != 1002 {
		t.Fatalf("Probe() = %d, want 1002 for a clock value below the previous number", seq)
	}

	boot.Set(5000)
	round(list)

	if seq, _ := list.Probe(); seq != 5000 {
		t.Fatalf("Probe() = %d, want 5000", seq)
	}
}

func TestRound_NoUpstreamsIsFailure(t *testing.T) {
	list, _, _ := newTestList(t)
	round(list)

	if seq, ok := list.Probe(); seq == 0 || ok {
		t.Errorf("Probe() = %d, %v, want a number and false", seq, ok)
	}
}

// Probe intervals: interval 1 s ±20 %, timeout 500 ms.
func TestDefaults_IntervalAndTimeout(t *testing.T) {
	list, _, _ := newFakeClockList(t, defaultOptions(), "a")
	round(list)

	assertDelays(t, list, defaultInterval)

	var budget time.Duration

	list.probe = func(ctx context.Context, _ *Upstream) (probeResult, error) {
		deadline, _ := ctx.Deadline()
		budget = time.Until(deadline)

		return probeResult{kind: probeReady}, nil
	}
	round(list)

	if budget <= defaultTimeout-100*time.Millisecond || budget > defaultTimeout {
		t.Errorf("probe budget = %v, want about %v", budget, defaultTimeout)
	}
}

// A round probes every upstream once: one failed probe fails the upstream
// in the round.
func TestProbeAll_OneProbe(t *testing.T) {
	for _, kind := range []probeKind{probeReady, probeNotReady, probeSoft, probeHard} {
		t.Run(kind.String(), func(t *testing.T) {
			var calls atomic.Int32

			probe := func(context.Context, *Upstream) (probeResult, error) {
				calls.Add(1)

				return probeResult{kind: kind}, nil
			}

			list, err := newList(nil, byAddr, append(defaultOptions(), withProbe(probe))...)
			if err != nil {
				t.Fatal(err)
			}

			out := list.probeAll(context.Background(), ups("a"))

			if got := calls.Load(); got != 1 || out[0].value.kind != kind {
				t.Errorf("calls = %d, result = %s, want 1, %s", got, out[0].value.kind, kind)
			}
		})
	}
}

// Packets are lost: the probe hangs until the timeout. The round fails after
// one timeout in the normal mode and in fail-step.
func TestRound_OneTimeout(t *testing.T) {
	const timeout = 100 * time.Millisecond

	for _, tt := range []struct {
		name     string
		failStep bool
	}{{name: "normal"}, {name: "fail-step", failStep: true}} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int64

			hang := func(ctx context.Context, _ *Upstream) (probeResult, error) {
				calls.Add(1)
				<-ctx.Done()

				return probeResult{kind: probeSoft}, nil
			}

			list, err := newList(nil, byAddr, append(defaultOptions(), WithHealthcheckTimeout(timeout), withProbe(hang))...)
			if err != nil {
				t.Fatal(err)
			}

			list.Reconcile(ups("a"))

			if tt.failStep {
				list.mu.Lock()
				list.failSince = list.now()
				list.mu.Unlock()
			}

			start := time.Now()
			list.round(context.Background(), list.beginRound())

			if elapsed := time.Since(start); elapsed < timeout || elapsed >= 2*timeout {
				t.Errorf("round took %v, want about %v", elapsed, timeout)
			}

			if got := calls.Load(); got != 1 {
				t.Errorf("probes = %d, want 1", got)
			}
		})
	}
}

// logLines decodes the JSON log lines whose message has the prefix.
func logLines(t *testing.T, buf *bytes.Buffer, prefix string) []map[string]any {
	t.Helper()

	var out []map[string]any

	for line := range strings.Lines(buf.String()) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}

		if msg, _ := rec["msg"].(string); strings.HasPrefix(msg, prefix) {
			out = append(out, rec)
		}
	}

	return out
}

// The proxy logs the start and the end of fail-step once per failure episode
// and logs nothing while the upstreams stay in the same state.
func TestFailStep_Logs(t *testing.T) {
	var buf bytes.Buffer

	addrs := []string{"a", "b"}
	list, probes, clock := newFakeClockList(t, defaultOptions(), addrs...)
	round(list)
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))

	for range 100 {
		round(list)
		clock.Advance(defaultInterval)
	}

	if buf.Len() != 0 {
		t.Fatalf("logs while all upstreams are ready:\n%s", buf.String())
	}

	// Two failure episodes: the first ends with a ready upstream, the second
	// with the end of the window.
	for range 2 {
		setAll(probes, probeSoft, addrs...)

		for range 3 {
			round(list)
			clock.Advance(failStepInterval)
		}

		probes.set("a", probeReady, time.Millisecond)
		round(list)
		clock.Advance(defaultInterval)
	}

	setAll(probes, probeSoft, addrs...)

	for elapsed := time.Duration(0); elapsed <= defaultWindow+time.Minute; elapsed += failStepInterval {
		round(list)
		clock.Advance(failStepInterval)
	}

	started := logLines(t, &buf, "fail-step started")
	if len(started) != 3 {
		t.Fatalf("fail-step started logged %d times, want 3:\n%s", len(started), buf.String())
	}

	for _, rec := range started {
		if rec["level"] != "warn" || rec["reason"] != "no upstream is ready" || rec["interval"] != failStepInterval.String() || rec["window"] != defaultWindow.String() {
			t.Errorf("fail-step started = %v, want level warn, reason, interval %v and window %v", rec, failStepInterval, defaultWindow)
		}
	}

	stopped := logLines(t, &buf, "fail-step stopped")
	if len(stopped) != 3 {
		t.Fatalf("fail-step stopped logged %d times, want 3:\n%s", len(stopped), buf.String())
	}

	wantStops := []struct{ reason, duration string }{
		{reason: "an upstream is ready", duration: (3 * failStepInterval).String()},
		{reason: "an upstream is ready", duration: (3 * failStepInterval).String()},
		{reason: "the fail-step window ended", duration: defaultWindow.String()},
	}
	for i, want := range wantStops {
		if stopped[i]["level"] != "warn" || stopped[i]["reason"] != want.reason || stopped[i]["duration"] != want.duration {
			t.Errorf("fail-step stopped #%d = %v, want level warn, reason %q and duration %s", i+1, stopped[i], want.reason, want.duration)
		}
	}
}

// Without fail-step there is nothing to log.
func TestFailStep_NoLogsWhenDisabled(t *testing.T) {
	var buf bytes.Buffer

	opts := append(defaultOptions(), WithFailStep(failStepInterval, 0))
	list, probes, clock := newFakeClockList(t, opts, "a")
	list.logger = log.NewLogger(log.WithOutput(&buf), log.WithLevel(slog.LevelInfo))

	probes.set("a", probeSoft, 0)

	for range 10 {
		round(list)
		clock.Advance(defaultInterval)
	}

	probes.set("a", probeReady, time.Millisecond)
	round(list)

	if got := logLines(t, &buf, "fail-step"); len(got) != 0 {
		t.Errorf("fail-step logs = %v, want none", got)
	}
}

// A zero fail-step window disables fail-step.
func TestFailStep_ZeroWindowDisables(t *testing.T) {
	opts := append(defaultOptions(), WithFailStep(failStepInterval, 0))
	list, probes, clock := newFakeClockList(t, opts, "a")

	probes.set("a", probeHard, 0)
	round(list)
	clock.Advance(100 * time.Millisecond)

	assertDelays(t, list, defaultInterval)
}

// Fail-step and its limits: all upstreams are not ready.
func TestFailStep_NobodyReady(t *testing.T) {
	addrs := []string{"a", "b", "c"}
	list, probes, _ := newFakeClockList(t, defaultOptions(), addrs...)
	round(list)

	setAll(probes, probeNotReady, addrs...)

	if !list.round(context.Background(), list.beginRound()) {
		t.Fatal("round() = false, want fail-step active")
	}

	assertDelays(t, list, failStepInterval)
}

// Fail-step and its limits: at least one upstream is ready.
func TestFailStep_OneReady(t *testing.T) {
	list, probes, _ := newFakeClockList(t, defaultOptions(), "a", "b", "c")
	probes.set("b", probeNotReady, time.Millisecond)
	probes.set("c", probeHard, 0)

	if list.round(context.Background(), list.beginRound()) {
		t.Fatal("round() = true, want no fail-step")
	}

	assertDelays(t, list, defaultInterval)
}

// Fail-step once per failure episode: a long CP outage and a new failure
// after recovery.
func TestFailStep_OncePerEpisode(t *testing.T) {
	addrs := []string{"a", "b", "c"}
	list, probes, clock := newFakeClockList(t, defaultOptions(), addrs...)
	round(list)

	outage := func() {
		t.Helper()

		setAll(probes, probeSoft, addrs...)

		for elapsed := time.Duration(0); elapsed < defaultWindow; elapsed += failStepInterval {
			round(list)
			assertDelays(t, list, failStepInterval)
			clock.Advance(failStepInterval)
		}

		// The window is over, the outage goes on: the normal interval for the
		// rest of the episode.
		for range 20 {
			round(list)
			assertDelays(t, list, defaultInterval)
			clock.Advance(defaultInterval)
		}
	}

	outage()

	probes.set("a", probeReady, time.Millisecond)
	round(list)
	assertDelays(t, list, defaultInterval)
	clock.Advance(time.Minute)

	outage()
}

// probeTimes records the start and the end of every probe.
type probeTimes struct {
	mu           sync.Mutex
	starts, ends []time.Time
}

// hang returns a probe that waits until its timeout.
func (p *probeTimes) hang(ctx context.Context, _ *Upstream) (probeResult, error) {
	p.mu.Lock()
	p.starts = append(p.starts, time.Now())
	p.mu.Unlock()

	<-ctx.Done()

	p.mu.Lock()
	p.ends = append(p.ends, time.Now())
	p.mu.Unlock()

	return probeResult{kind: probeSoft}, nil
}

// In normal mode a round that ends before the interval does not move the
// schedule: the next round starts the interval after the start of the
// previous round.
func TestRunLoop_ScheduleFromRoundStart(t *testing.T) {
	const (
		interval = 200 * time.Millisecond
		timeout  = 100 * time.Millisecond
	)

	var p probeTimes

	list, err := NewList(ups("a"),
		WithHealthcheckInterval(interval),
		WithHealthcheckTimeout(timeout),
		WithFailStep(failStepInterval, 0),
		withProbe(p.hang),
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	list.Shutdown()

	if len(p.starts) < 4 {
		t.Fatalf("rounds = %d, want at least 4", len(p.starts))
	}

	for i := 1; i < len(p.starts); i++ {
		if gap := p.starts[i].Sub(p.starts[i-1]); gap < interval-10*time.Millisecond || gap >= interval+timeout-20*time.Millisecond {
			t.Errorf("round %d started %v after the previous one, want about %v", i, gap, interval)
		}
	}
}

// In fail-step a round that takes longer than the fail-step interval ends
// before the next round starts.
func TestRunLoop_FailStepWaitsForRoundEnd(t *testing.T) {
	const timeout = 100 * time.Millisecond

	var p probeTimes

	list, err := NewList(ups("a"),
		WithHealthcheckInterval(time.Hour),
		WithHealthcheckTimeout(timeout),
		WithFailStep(20*time.Millisecond, time.Hour),
		withProbe(p.hang),
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	list.Shutdown()

	if len(p.starts) < 3 {
		t.Fatalf("rounds = %d, want at least 3 in fail-step", len(p.starts))
	}

	for i := 1; i < len(p.starts); i++ {
		if p.starts[i].Before(p.ends[i-1]) {
			t.Errorf("round %d started %v before the previous round ended", i, p.ends[i-1].Sub(p.starts[i]))
		}
	}

	if seq, ok := list.Probe(); seq == 0 || ok {
		t.Errorf("Probe() = %d, %v, want failed rounds", seq, ok)
	}
}

// One upstream hangs until the timeout in every round, and another upstream
// answers ready at once. The age of the ready answer stays below the interval
// with the largest jitter plus the timeout.
func TestRunLoop_LastReadyAgoWithHungUpstream(t *testing.T) {
	const (
		interval = 200 * time.Millisecond
		timeout  = 100 * time.Millisecond
	)

	var p probeTimes

	probe := func(ctx context.Context, u *Upstream) (probeResult, error) {
		if u.Addr == "a" {
			return probeResult{kind: probeReady, latency: time.Millisecond}, nil
		}

		return p.hang(ctx, u)
	}

	list, err := NewList(ups("a", "b"),
		WithHealthcheckInterval(interval),
		WithHealthcheckTimeout(timeout),
		WithHealthCheckJitter(defaultJitter),
		WithFailStep(failStepInterval, defaultWindow),
		withProbe(probe),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(list.Shutdown)

	waitReady(t, list, "a")

	limit := time.Duration(float64(interval)*(1+defaultJitter)) + timeout

	var worst int64

	for deadline := time.Now().Add(1500 * time.Millisecond); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		worst = max(worst, lastReadyAgo(t, list, "a"))
	}

	if worst > limit.Milliseconds() {
		t.Errorf("lastReadyAgoMs of the ready upstream = %d, want <= %d", worst, limit.Milliseconds())
	}

	t.Logf("lastReadyAgoMs of the ready upstream: <= %d (limit %d)", worst, limit.Milliseconds())
}
