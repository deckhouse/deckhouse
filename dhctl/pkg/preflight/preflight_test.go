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

package preflightnew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// testContext returns a context carrying a logger that writes everything into buf, so a test can
// assert on the records the user would see.
func testContext(t *testing.T) (context.Context, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return dhlog.ToContext(t.Context(), logger), buf
}

// passing/failing build the checks the tests are written against. Every one records how many
// times its body ran, which is what the retry assertions read.
type recorder struct {
	mu    sync.Mutex
	runs  map[CheckName]int
	order []CheckName
}

func newRecorder() *recorder { return &recorder{runs: map[CheckName]int{}} }

func (r *recorder) count(name CheckName) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[name]
}

func (r *recorder) check(name CheckName, phase Phase, body func() (string, error)) Check {
	return Check{
		Name:        name,
		Description: string(name) + " holds",
		Phase:       phase,
		Retry:       NoRetry,
		Run: func(context.Context) (string, error) {
			r.mu.Lock()
			r.runs[name]++
			r.order = append(r.order, name)
			r.mu.Unlock()
			return body()
		},
	}
}

func passing(r *recorder, name CheckName) Check {
	return r.check(name, PhasePreInfra, func() (string, error) { return "", nil })
}

func failing(r *recorder, name CheckName, err error) Check {
	return r.check(name, PhasePreInfra, func() (string, error) { return "", err })
}

// TestRunReportsEveryFailureOfThePhase is the point of the collect-all runner: three mistakes in
// one configuration have to cost one run, not three. The old runner returned at the first error,
// and on a cloud cluster each of the runs that bought was paid for in infrastructure.
func TestRunReportsEveryFailureOfThePhase(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	first := errors.New("first is wrong")
	second := errors.New("second is wrong")

	p := New(NewSuite(
		failing(r, "check-one", first),
		passing(r, "check-two"),
		failing(r, "check-three", second),
	))

	err := p.Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("a phase with two failed checks must return an error")
	}
	for _, name := range []CheckName{"check-one", "check-two", "check-three"} {
		if got := r.count(name); got != 1 {
			t.Errorf("%s ran %d times, want 1: every check of the phase runs", name, got)
		}
	}
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Errorf("both causes must stay reachable through errors.Is, got %v", err)
	}
	if !strings.Contains(err.Error(), "2 preflight checks failed") {
		t.Errorf("the report must open with the count, got:\n%s", err)
	}
}

// TestFailFastStopsAtTheFirstFailure keeps the old behaviour available to anyone who wants it.
func TestFailFastStopsAtTheFirstFailure(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		failing(r, "check-one", errors.New("nope")),
		passing(r, "check-two"),
	))
	p.SetFailFast(true)

	if err := p.Run(ctx, PhasePreInfra); err == nil {
		t.Fatal("want an error")
	}
	if got := r.count("check-two"); got != 0 {
		t.Errorf("check-two ran %d times, want 0 under --preflight-fail-fast", got)
	}
}

// TestDependentCheckIsBlockedNotRun covers the softer of the two relationships: sudo-installed
// failing makes sudo-allowed pointless, but says nothing about python or the hostname, so the
// phase carries on and only the dependent is reported blocked.
func TestDependentCheckIsBlockedNotRun(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		failing(r, "sudo-installed", errors.New("sudo is not installed")),
		passing(r, "sudo-allowed").After("sudo-installed"),
		passing(r, "python-modules"),
	))

	err := p.Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}
	if got := r.count("sudo-allowed"); got != 0 {
		t.Errorf("sudo-allowed ran %d times, want 0 while its dependency is failing", got)
	}
	if got := r.count("python-modules"); got != 1 {
		t.Errorf("python-modules ran %d times, want 1: it does not depend on sudo", got)
	}
	if !strings.Contains(buf.String(), "sudo-allowed blocked: sudo-installed did not pass") {
		t.Errorf("the blocked record must name what blocked it, got:\n%s", buf)
	}
	if strings.Contains(err.Error(), "sudo-allowed") {
		t.Errorf("a blocked check is not a failure and must stay out of the report:\n%s", err)
	}
}

// TestCriticalPathFailureStopsThePhase is the harder relationship: the SSH connection every node
// check is asked over. Nothing after it can be asked, and the bootstrap would not get past it
// either, so the phase ends on the failure — which leaves it as the last thing on the screen
// instead of the first line above a page of records saying nothing happened.
func TestCriticalPathFailureStopsThePhase(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	login := failing(r, "static-ssh-credential", errors.New("permission denied"))
	login.StopsPhaseOnFailure = true

	p := New(NewSuite(
		passing(r, "static-single-ssh-host"),
		login,
		passing(r, "sudo-allowed"),
		passing(r, "python-modules"),
		passing(r, "time-drift"),
	))

	err := p.Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, name := range []CheckName{"sudo-allowed", "python-modules", "time-drift"} {
		if got := r.count(name); got != 0 {
			t.Errorf("%s ran %d times, want 0 after the connection was refused", name, got)
		}
	}

	out := buf.String()
	if strings.Contains(out, "sudo-allowed") {
		t.Errorf("no record may be printed for a check the phase never reached, got:\n%s", out)
	}
	// The tally rides in the report, not on a line of its own: the process box already prints
	// the phase as failed, and a second line naming the same phase said it twice.
	if !strings.Contains(err.Error(), "3 not run after the phase stopped") {
		t.Errorf("the report must account for the checks that were not reached, got:\n%s", err)
	}
	if !strings.Contains(err.Error(), "1 preflight check failed") {
		t.Errorf("the report carries the one failure that matters, got:\n%s", err)
	}
	// The process box opens and closes with the phase name, which is fine. What must not be
	// there is a compacted record naming it again: those are what the terminal keeps, and two
	// of them read as two failures.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "compact=true") && strings.Contains(line, "Preflight checks: configuration") {
			t.Errorf("the phase is named twice in the compacted view:\n%s", line)
		}
	}
}

// TestNotApplicableIsNeitherPassNorFailure covers the outcome the runner used to have no room
// for: a check with nothing to look at printed the same ✓ as one that had looked and approved.
func TestNotApplicableIsNeitherPassNorFailure(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		r.check("bastion-availability", PhasePreInfra, func() (string, error) {
			return "", NotApplicable("no --ssh-bastion-host")
		}),
	))

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("a not-applicable check is not a failure: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "↷ bastion-availability not applicable: no --ssh-bastion-host") {
		t.Errorf("want the not-applicable record with its reason, got:\n%s", out)
	}
	if !strings.Contains(out, "0 passed") {
		t.Errorf("a not-applicable check asserts nothing and must not be counted as passed, got:\n%s", out)
	}
}

// TestSkippedByFlagAndDisabledByDhctlReadDifferently is the distinction the single ✓ hid: a
// check the user turned off, and a check dhctl turned off because it did not apply.
func TestSkippedByFlagAndDisabledByDhctlReadDifferently(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		passing(r, "check-one"),
		passing(r, "check-two"),
	))
	p.DisableCheck("check-one")
	p.DisableCheckWithReason("check-two", "an immutable master has no sshd")

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "– check-one skipped: --preflight-skip-check=check-one") {
		t.Errorf("a user-skipped check must name the flag that skipped it, got:\n%s", out)
	}
	if !strings.Contains(out, "↷ check-two not applicable: an immutable master has no sshd") {
		t.Errorf("a dhctl-disabled check must state its reason, got:\n%s", out)
	}
	if r.count("check-one")+r.count("check-two") != 0 {
		t.Error("a disabled check must not run")
	}
}

// TestSkippingEveryCheckByNameIsNotSkipAll: the collapsed line names a flag, so it may only be
// printed when that flag is what turned the checks off.
func TestSkippingEveryCheckByNameIsNotSkipAll(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(passing(r, "check-one"), passing(r, "check-two")))
	p.DisableChecks("check-one", "check-two")

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "--preflight-skip-all-checks") {
		t.Errorf("a flag the user did not pass must not be named, got:\n%s", out)
	}
	if !strings.Contains(out, "– check-one skipped: --preflight-skip-check=check-one") {
		t.Errorf("each check must name the flag that actually skipped it, got:\n%s", out)
	}
}

// TestSkipAllCollapsesToOneLine: expanding --preflight-skip-all-checks used to print a "skipped"
// record for each of thirty-odd names, which says nothing the one line does not.
func TestSkipAllCollapsesToOneLine(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(passing(r, "check-one"), passing(r, "check-two"), passing(r, "check-three")))
	p.DisableChecks("check-one", "check-two", "check-three")
	p.SetSkippedAll(true)

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "– all 3 checks skipped: --preflight-skip-all-checks") {
		t.Errorf("want the collapsed line, got:\n%s", out)
	}
	if strings.Contains(out, "check-two skipped") {
		t.Errorf("the per-check records must not be printed as well, got:\n%s", out)
	}
}

// TestRetryIsAnnouncedAndBounded covers the fifteen quiet seconds: the attempts are now Info
// records, so a retrying check no longer looks exactly like a hung one.
func TestRetryIsAnnouncedAndBounded(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	check := r.check("flaky", PhasePreInfra, func() (string, error) { return "", errors.New("i/o timeout") })
	check.Retry = RetryPolicy{Attempts: 3, Options: []backoff.ExponentialBackOffOpts{
		backoff.WithInitialInterval(time.Millisecond),
		backoff.WithMultiplier(1),
	}}

	p := New(NewSuite(check))
	if err := p.Run(ctx, PhasePreInfra); err == nil {
		t.Fatal("want an error")
	}
	if got := r.count("flaky"); got != 3 {
		t.Errorf("ran %d times, want 3", got)
	}
	out := buf.String()
	if !strings.Contains(out, "↻ flaky attempt 1/3 failed: i/o timeout") {
		t.Errorf("each retry must be announced, got:\n%s", out)
	}
	if !strings.Contains(out, "failed after 3 attempts") {
		t.Errorf("the record must say how many attempts were spent, got:\n%s", out)
	}
}

// TestPermanentFailureIsNotRetried: an HTTP 401 or a missing sudoers rule answers the same way
// every time, and spending five attempts on it only delays the report.
func TestPermanentFailureIsNotRetried(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	cause := errors.New("registry rejected the credentials")
	check := r.check("registry-credentials", PhasePreInfra, func() (string, error) {
		return "", Permanent(cause)
	})
	check.Retry = NetworkRetry

	p := New(NewSuite(check))
	err := p.Run(ctx, PhasePreInfra)
	if !errors.Is(err, cause) {
		t.Fatalf("the cause must survive Permanent, got %v", err)
	}
	if got := r.count("registry-credentials"); got != 1 {
		t.Errorf("ran %d times, want 1", got)
	}
	if !strings.Contains(buf.String(), "permanent failure, not retried") {
		t.Errorf("the record must say it was not retried, got:\n%s", buf)
	}
}

// TestAttemptTimeoutIsPerAttempt: a check that accepts the connection and never answers used to
// hang the run, because DefaultPreflightCheckTimeout was declared and referenced nowhere.
func TestAttemptTimeoutIsPerAttempt(t *testing.T) {
	ctx, _ := testContext(t)

	check := Check{
		Name:        "hangs",
		Description: "answers",
		Phase:       PhasePreInfra,
		Retry:       NoRetry,
		Timeout:     10 * time.Millisecond,
		Run: func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}

	err := New(NewSuite(check)).Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("want a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after 10ms") {
		t.Errorf("the failure must name the deadline it hit, got:\n%s", err)
	}
}

// TestSuccessDetailReplacesTheDescription: the ✓ line is worth reading only when it says
// something about this cluster rather than repeating the assertion the check is named for.
func TestSuccessDetailReplacesTheDescription(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(r.check("registry-credentials", PhasePreInfra, func() (string, error) {
		return `authenticated to https://registry.deckhouse.ru as "license-token"`, nil
	})))

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `✓ registry-credentials authenticated to https://registry.deckhouse.ru as`) {
		t.Errorf("want the reported detail on the record line, got:\n%s", buf)
	}
}

// TestOnlyTheRequestedPhaseRuns guards the filter the phase tree depends on.
func TestOnlyTheRequestedPhaseRuns(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		passing(r, "pre-check"),
		r.check("post-check", PhasePostInfra, func() (string, error) { return "", nil }),
	))

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.count("post-check") != 0 {
		t.Error("a post-infra check must not run in the pre-infra phase")
	}
	if r.count("pre-check") != 1 {
		t.Error("the pre-infra check must run")
	}
}

// TestInvalidNameIsRefusedBeforeAnythingRuns: a malformed name is a programming error, and it is
// cheaper to hear about it before the run than after half the checks have executed.
func TestInvalidNameIsRefusedBeforeAnythingRuns(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	p := New(NewSuite(
		passing(r, "good-name"),
		passing(r, "Bad_Name"),
	))

	if err := p.Run(ctx, PhasePreInfra); err == nil {
		t.Fatal("want an error for the invalid name")
	}
	if r.count("good-name") != 0 {
		t.Error("nothing may run when a name is invalid")
	}
}

// fakeCache is the state cache reduced to what the runner asks of it.
type fakeCache struct {
	entries map[string][]byte
	saves   int
}

func newFakeCache() *fakeCache { return &fakeCache{entries: map[string][]byte{}} }

func (c *fakeCache) Save(_ context.Context, key string, data []byte) error {
	c.entries[key] = data
	c.saves++
	return nil
}

func (c *fakeCache) InCache(_ context.Context, key string) (bool, error) {
	_, ok := c.entries[key]
	return ok, nil
}

func (c *fakeCache) Load(_ context.Context, key string) ([]byte, error) {
	data, ok := c.entries[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return data, nil
}

// TestOnlyCacheableChecksAreRemembered is the fix for the cached ✓ about a machine that no
// longer exists: the key is built from the configuration, so only a check that is a function of
// the configuration may be answered from it.
func TestOnlyCacheableChecksAreRemembered(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	pure := passing(r, "cidr-intersection")
	pure.Cacheable = true
	node := passing(r, "sudo-allowed")

	run := func(cache *fakeCache) {
		p := New(NewSuite(pure, node))
		p.UseCache(cache)
		p.SetCacheSalt("confighash")
		if err := p.Run(ctx, PhasePreInfra); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	cache := newFakeCache()
	run(cache)
	run(cache)

	if got := r.count("cidr-intersection"); got != 1 {
		t.Errorf("the cacheable check ran %d times, want 1: the second run must come from the cache", got)
	}
	if got := r.count("sudo-allowed"); got != 2 {
		t.Errorf("the node check ran %d times, want 2: it must never be answered from the cache", got)
	}
}

// TestCacheSaltInvalidatesRememberedResults: change the configuration and the remembered answer
// is about a different question.
func TestCacheSaltInvalidatesRememberedResults(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	check := passing(r, "cidr-intersection")
	check.Cacheable = true
	cache := newFakeCache()

	for _, salt := range []string{"hash-one", "hash-two"} {
		p := New(NewSuite(check))
		p.UseCache(cache)
		p.SetCacheSalt(salt)
		if err := p.Run(ctx, PhasePreInfra); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := r.count("cidr-intersection"); got != 2 {
		t.Errorf("ran %d times, want 2: a different salt is a different question", got)
	}
}

// TestDisableCacheIgnoresAndDoesNotWrite covers --preflight-no-cache.
func TestDisableCacheIgnoresAndDoesNotWrite(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	check := passing(r, "cidr-intersection")
	check.Cacheable = true
	cache := newFakeCache()

	for range 2 {
		p := New(NewSuite(check))
		p.UseCache(cache)
		p.SetCacheSalt("confighash")
		p.DisableCache()
		if err := p.Run(ctx, PhasePreInfra); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := r.count("cidr-intersection"); got != 2 {
		t.Errorf("ran %d times, want 2 with the cache disabled", got)
	}
	if cache.saves != 0 {
		t.Errorf("wrote %d cache entries, want 0", cache.saves)
	}
}

// TestNotApplicableIsNotRemembered: there is nothing to remember about a check that found
// nothing to look at, and a later run may well find something.
func TestNotApplicableIsNotRemembered(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	check := r.check("bastion-availability", PhasePreInfra, func() (string, error) {
		return "", NotApplicable("no --ssh-bastion-host")
	})
	check.Cacheable = true

	cache := newFakeCache()
	p := New(NewSuite(check))
	p.UseCache(cache)
	p.SetCacheSalt("confighash")
	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cache.saves != 0 {
		t.Errorf("wrote %d cache entries, want 0", cache.saves)
	}
}

// TestTitleOverride is what lets abort say what it is rather than borrow the phase's name.
func TestTitleOverride(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(r.check("sudo-allowed", PhasePostInfra, func() (string, error) { return "", nil })))
	p.SetTitle("Preflight checks: abort")

	if err := p.Run(ctx, PhasePostInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Preflight checks: abort") {
		t.Errorf("want the overridden title, got:\n%s", buf)
	}
}

// TestCachedRecordSaysHowOldTheAnswerIs: a remembered pass is only as trustworthy as it is
// recent, and the record used to print the same ✓ as a check that had just run.
func TestCachedRecordSaysHowOldTheAnswerIs(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	check := passing(r, "cidr-intersection")
	check.Cacheable = true

	cache := newFakeCache()
	// A pass recorded twelve minutes ago, in the format the runner writes.
	cache.entries["preflight-confighash-cidr-intersection"] =
		[]byte(time.Now().Add(-12 * time.Minute).Format(time.RFC3339))

	p := New(NewSuite(check))
	p.UseCache(cache)
	p.SetCacheSalt("confighash")
	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.count("cidr-intersection"); got != 0 {
		t.Errorf("ran %d times, want 0: the answer was remembered", got)
	}
	// The stored timestamp is RFC3339, which has no sub-second part, so the age lands on either
	// side of the minute depending on when the test ran.
	if !strings.Contains(buf.String(), "(cached, 12m") {
		t.Errorf("want the age of the remembered answer on the record, got:\n%s", buf)
	}
}

// TestCacheEntryWithoutATimestampStillCounts: entries written by an earlier dhctl hold the
// string "yes", and must not be mistaken for a miss.
func TestCacheEntryWithoutATimestampStillCounts(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	check := passing(r, "cidr-intersection")
	check.Cacheable = true

	cache := newFakeCache()
	cache.entries["preflight-confighash-cidr-intersection"] = []byte("yes")

	p := New(NewSuite(check))
	p.UseCache(cache)
	p.SetCacheSalt("confighash")
	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.count("cidr-intersection"); got != 0 {
		t.Errorf("ran %d times, want 0", got)
	}
	out := buf.String()
	if !strings.Contains(out, "(cached)") || strings.Contains(out, "ago") {
		t.Errorf("want a plain cached record with no age, got:\n%s", out)
	}
}

// TestUnskippableCheckIsNotSkipped is the runner's half of the guarantee: a check that says it
// cannot be skipped runs whatever the caller passed. The flag layer refuses the name, but
// --preflight-skip-all-checks reaches the runner as a list of every name there is, and an
// operator reaching for that flag is usually trying to get past something unrelated.
func TestUnskippableCheckIsNotSkipped(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	guard := failing(r, "immutable-registry-mode", errors.New("registry mode is Managed"))
	guard.CannotBeSkipped = true

	p := New(NewSuite(guard, passing(r, "ordinary-check")))
	p.DisableChecks("immutable-registry-mode", "ordinary-check")
	p.SetSkippedAll(true)

	err := p.Run(ctx, PhasePreInfra)
	if err == nil {
		t.Fatal("the guard must still fail the phase")
	}
	if got := r.count("immutable-registry-mode"); got != 1 {
		t.Errorf("the guard ran %d times, want 1", got)
	}
	if got := r.count("ordinary-check"); got != 0 {
		t.Errorf("an ordinary check must still be skipped, ran %d times", got)
	}
	if !strings.Contains(err.Error(), "skip: cannot be skipped") {
		t.Errorf("the report must say there is no flag for it, got:\n%s", err)
	}
	if !strings.Contains(buf.String(), "– ordinary-check skipped") {
		t.Errorf("the ordinary check must be reported as skipped, got:\n%s", buf)
	}
}

// TestAddSuiteAppendsAndIgnoresNil: the bootstrap builds its suite list conditionally — the
// immutable suite exists only for an immutable master — and a branch that produces nothing must
// not leave a nil in the list for the runner to walk into.
func TestAddSuiteAppendsAndIgnoresNil(t *testing.T) {
	ctx, buf := testContext(t)
	r := newRecorder()

	p := New(NewSuite(passing(r, "first")))
	p.AddSuite(NewSuite(passing(r, "second")))
	p.AddSuite(nil)

	if err := p.Run(ctx, PhasePreInfra); err != nil {
		t.Fatalf("both suites pass: %v", err)
	}
	if got := r.count("first"); got != 1 {
		t.Errorf("want the check of the original suite run once, got %d", got)
	}
	if got := r.count("second"); got != 1 {
		t.Errorf("want the check of the added suite run once, got %d", got)
	}
	if out := buf.String(); !strings.Contains(out, "2 passed") {
		t.Errorf("want both checks counted, got:\n%s", out)
	}
}

// TestDisabledReasonTellsTheTwoApart: a check the operator skipped with a flag and one dhctl
// turned off itself read the same in the report until each carried its reason. DisabledReason is
// how the bootstrap asks afterwards which it was.
func TestDisabledReasonTellsTheTwoApart(t *testing.T) {
	r := newRecorder()
	p := New(NewSuite(passing(r, "by-flag"), passing(r, "by-dhctl")))

	p.DisableCheck("by-flag")
	p.DisableCheckWithReason("by-dhctl", "an immutable master runs no sshd")

	// The empty reason is what the user's own flag looks like, and it is the difference the
	// report renders.
	if reason, disabled := p.DisabledReason("by-flag"); !disabled || reason != "" {
		t.Errorf("want a flag-disabled check with no reason, got %q disabled=%v", reason, disabled)
	}
	if reason, disabled := p.DisabledReason("by-dhctl"); !disabled || reason != "an immutable master runs no sshd" {
		t.Errorf("want dhctl's own reason, got %q disabled=%v", reason, disabled)
	}
	if _, disabled := p.DisabledReason("never-mentioned"); disabled {
		t.Error("a check nobody disabled must not report itself disabled")
	}
}

// The brake belongs to the connection, not to whoever asked first. ssh-credential carries
// StopsPhaseOnFailure, and skipping it by name used to take the brake with it: the connection was
// just as absent, and every check after it paid lib-connection's own two-minute retry loop to
// find that out. Bought live on 2026-09-16 with --preflight-skip-check=ssh-credential and a wrong
// --ssh-user: "Get SSH client FAILED (118.61 seconds)", then the next check opening another one.
func TestAFindingCanStopThePhase(t *testing.T) {
	ctx, _ := testContext(t)
	r := newRecorder()

	connection := r.check("cloud-api-accessibility", PhasePreInfra, func() (string, error) {
		return "", &Failure{Observed: "no connection", StopsPhase: true}
	})
	overIt := r.check("node-hostname", PhasePreInfra, func() (string, error) {
		return "", nil
	})

	p := New(NewSuite(connection, overIt))
	if err := p.Run(ctx, PhasePreInfra); err == nil {
		t.Fatal("the phase must fail")
	}

	if got := r.count("node-hostname"); got != 0 {
		t.Errorf("the checks after an unusable connection must not be asked at all, ran %d times", got)
	}
}
