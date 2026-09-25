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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// skippedByFlagReason is the DisabledReason of a check the user turned off, as opposed to one
// dhctl turned off for it (DisableCheckWithReason).
const skippedByFlagReason = ""

type Preflight struct {
	suites []Suite
	// disabled maps a check to why it is off. The empty reason means the user's flag.
	disabled  map[CheckName]string
	cache     cache
	cacheSalt string
	noCache   bool
	failFast  bool
	// skippedAll records that the whole phase was turned off by one flag rather than by naming
	// every check, which is what the collapsed record line reports.
	skippedAll bool
	// title overrides the phase title, for a runner whose phase says less than the command
	// does: abort runs the post-infra phase but has no infrastructure phase to be after.
	title string
}

type cache interface {
	Save(context.Context, string, []byte) error
	InCache(context.Context, string) (bool, error)
	Load(context.Context, string) ([]byte, error)
}

func New(suites ...Suite) *Preflight {
	return &Preflight{
		suites:   append([]Suite(nil), suites...),
		disabled: make(map[CheckName]string),
	}
}

func (p *Preflight) UseCache(cache cache) {
	p.cache = cache
}

func (p *Preflight) SetCacheSalt(salt string) {
	p.cacheSalt = salt
}

// DisableCache makes this run ignore and not write the cache, for --preflight-no-cache and for
// the commands whose checks must not inherit a bootstrap's answers.
func (p *Preflight) DisableCache() {
	p.noCache = true
}

// SetFailFast restores the pre-collect-all behaviour: stop at the first failed check.
func (p *Preflight) SetFailFast(failFast bool) {
	p.failFast = failFast
}

// SetSkippedAll tells the runner that --preflight-skip-all-checks was passed, so it reports one
// line naming that flag instead of a "skipped" record for each of thirty-odd names.
func (p *Preflight) SetSkippedAll(skippedAll bool) {
	p.skippedAll = skippedAll
}

// SetTitle overrides the phase title of the process box.
func (p *Preflight) SetTitle(title string) {
	p.title = title
}

func (p *Preflight) AddSuite(suite Suite) {
	if suite == nil {
		return
	}
	p.suites = append(p.suites, suite)
}

// DisableCheck turns a check off on the user's behalf: the report says which flag did it.
func (p *Preflight) DisableCheck(name string) {
	p.disabled[CheckName(name)] = skippedByFlagReason
}

// DisabledReason reports whether a check is off in this runner, and why. The empty reason means
// the user's flag turned it off; anything else is dhctl's own doing, via DisableCheckWithReason.
func (p *Preflight) DisabledReason(name string) (string, bool) {
	reason, ok := p.disabled[CheckName(name)]
	return reason, ok
}

func (p *Preflight) DisableChecks(names ...string) {
	for _, name := range names {
		p.DisableCheck(name)
	}
}

// DisableCheckWithReason turns a check off on dhctl's own behalf and says why — an immutable
// master has no sshd for the cloud API check to tunnel through, and the reader is owed that
// sentence rather than a line indistinguishable from one they skipped themselves.
func (p *Preflight) DisableCheckWithReason(name, reason string) {
	if reason == skippedByFlagReason {
		reason = "disabled by dhctl"
	}
	p.disabled[CheckName(name)] = reason
}

func (p *Preflight) Run(ctx context.Context, phase Phase) error {
	checks, err := p.prepareChecks(phase)
	if err != nil {
		return err
	}

	title := p.titleFor(phase)
	runFunc := func(ctx context.Context) error {
		return p.runChecks(ctx, checks, title)
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), title, runFunc)
}

func (p *Preflight) titleFor(phase Phase) string {
	if p.title != "" {
		return p.title
	}
	return phase.FormatString()
}

// runChecks runs every check of the phase and reports them together. It used to return the first
// error it hit, which meant three mistakes in one configuration cost three bootstrap runs — and
// on a cloud cluster each of those runs paid for the infrastructure the post-infra phase sits
// behind. --preflight-fail-fast brings the old behaviour back for anyone who wants it.
func (p *Preflight) runChecks(ctx context.Context, checks []Check, title string) error {
	log := dhlog.FromContext(ctx)

	if line, ok := p.allSkippedByFlag(checks); ok {
		log.InfoContext(ctx, line, dhlog.ShowInCompacted())
		log.WarnContext(ctx, "Preflight checks skipped by --preflight-skip-all-checks. The run may fail later for a reason the checks would have named.")
		return nil
	}

	results := make([]Result, 0, len(checks))
	// broken holds the checks a dependent must not be run after: the ones that failed, and the
	// ones already blocked by those, so a chain of dependencies collapses to its first cause.
	broken := make(map[CheckName]struct{}, len(checks))
	notRun := 0

	for i, check := range checks {
		result := p.evaluate(ctx, check, broken)
		results = append(results, result)
		log.InfoContext(ctx, result.line(), dhlog.ShowInCompacted())

		if result.Status == StatusFailed || result.Status == StatusBlocked {
			broken[check.Name] = struct{}{}
		}
		if result.Status != StatusFailed {
			continue
		}

		// A check on the critical path, a finding that leaves the rest unaskable, or
		// --preflight-fail-fast. Either way the phase ends on the failure, which is then the last
		// thing the reader is looking at rather than the first line above a page of records
		// saying nothing happened.
		if check.StopsPhaseOnFailure || stopsPhase(result.Err) || p.failFast {
			notRun = len(checks) - i - 1
			break
		}
	}

	err := newPhaseError(title, results, notRun)
	if err != nil {
		// Nothing more here. RunProcess prints the phase as failed on its own, so a milestone
		// naming the same phase again put two identical FAILED lines one under the other; the
		// tally that made the second one worth reading is in the report instead.
		return err
	}

	// A milestone rather than an ordinary record: the per-check lines live in the phase box,
	// which the interactive renderer tears down when the phase ends, and the one line worth
	// keeping in the closing summary is this tally.
	log.InfoContext(ctx, fmt.Sprintf("%s — %s", title, summarize(results, notRun)),
		dhlog.BadgeSuccess(), dhlog.ShowInCompacted())

	return nil
}

// evaluate decides one check's outcome without running anything it does not have to.
func (p *Preflight) evaluate(ctx context.Context, check Check, broken map[CheckName]struct{}) Result {
	result := Result{
		Name:                  check.Name,
		Description:           check.Description,
		CannotBeSkipped:       check.CannotBeSkipped,
		CannotBeSkippedReason: check.CannotBeSkippedReason,
	}

	if check.Disabled {
		// A check dhctl turned off for a reason it can state did not apply here; a check the
		// user turned off with a flag was applicable and is being ignored. Those are different
		// things to read in a report, so they get different glyphs and different words.
		if check.DisabledReason != skippedByFlagReason {
			result.Status = StatusNotApplicable
			result.Detail = check.DisabledReason
			return result
		}
		result.Status = StatusSkipped
		result.Detail = fmt.Sprintf("--preflight-skip-check=%s", check.Name)
		return result
	}

	if dep, blocked := firstBroken(check.DependsOn, broken); blocked {
		result.Status = StatusBlocked
		result.Detail = fmt.Sprintf("%s did not pass", dep)
		return result
	}

	if age, ok := p.cached(ctx, check); ok {
		result.Status = StatusCached
		result.Age = age
		return result
	}

	start := time.Now()
	detail, attempt, err := p.retry(ctx, check)
	result.Attempts = attempt.count
	result.Permanent = attempt.permanent
	result.Elapsed = time.Since(start)

	switch {
	case err == nil:
		result.Status = StatusPassed
		result.Detail = detail
		p.store(ctx, check)
	case errors.Is(err, ErrNotApplicable):
		// Not a pass and not a failure: the check found nothing to look at, so it is neither
		// remembered nor counted among the assertions that hold.
		result.Status = StatusNotApplicable
		result.Detail = notApplicableReason(err)
	default:
		result.Status = StatusFailed
		result.Err = err
	}
	return result
}

// stopsPhase reports whether the failure itself says the rest of the phase cannot be asked.
func stopsPhase(err error) bool {
	var failure *Failure
	return errors.As(err, &failure) && failure.StopsPhase
}

func firstBroken(deps []CheckName, broken map[CheckName]struct{}) (CheckName, bool) {
	for _, dep := range deps {
		if _, ok := broken[dep]; ok {
			return dep, true
		}
	}
	return "", false
}

// allSkippedByFlag collapses --preflight-skip-all-checks into one line. Printing thirty-odd
// identical "skipped" records says nothing the single line does not.
func (p *Preflight) allSkippedByFlag(checks []Check) (string, bool) {
	if !p.skippedAll || len(checks) < 2 {
		return "", false
	}
	for _, check := range checks {
		if !check.Disabled || check.DisabledReason != skippedByFlagReason {
			return "", false
		}
	}
	return fmt.Sprintf("– all %d checks skipped: --preflight-skip-all-checks", len(checks)), true
}

func (p *Preflight) prepareChecks(phase Phase) ([]Check, error) {
	var checks []Check
	for _, suite := range p.suites {
		if suite == nil {
			continue
		}
		for _, check := range suite.Checks() {
			if err := check.Name.Validate(); err != nil {
				return nil, err
			}
			if check.Phase != phase {
				continue
			}
			if reason, ok := p.disabled[check.Name]; ok && !check.CannotBeSkipped {
				check.Disable()
				check.DisabledReason = reason
			}
			checks = append(checks, check)
		}
	}
	return checks, nil
}

// attemptStats is what the retry loop reports back about how the body was run, so the record
// line can say "failed after 3 attempts" or "permanent failure, not retried" rather than leaving
// the reader to guess whether the 15 quiet seconds were a retry or a hang.
type attemptStats struct {
	count     int
	permanent bool
}

func (p *Preflight) retry(ctx context.Context, check Check) (string, attemptStats, error) {
	attempts := check.Retry.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	timeout := check.Timeout
	if timeout <= 0 {
		timeout = DefaultPreflightCheckTimeout
	}

	var bo backoff.BackOff = backoff.NewExponentialBackOff(check.Retry.Options...)
	bo = backoff.WithMaxRetries(bo, uint64(attempts-1))
	bo = backoff.WithContext(bo, ctx)

	log := dhlog.FromContext(ctx)
	stats := attemptStats{}
	detail := ""

	operation := func() error {
		stats.count++
		// The deadline is per attempt, not per check: a check that is retried three times is
		// allowed three timeouts, and a timeout counts as one of its attempts.
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		out, err := check.Run(attemptCtx)
		if err == nil {
			detail = out
			return nil
		}
		if errors.Is(err, ErrNotApplicable) {
			// Nothing to look at will still be nothing to look at on the next attempt.
			stats.permanent = true
			return backoff.Permanent(err)
		}
		var permanent *backoff.PermanentError
		if errors.As(err, &permanent) {
			stats.permanent = true
			return err
		}
		if attemptCtx.Err() != nil && ctx.Err() == nil {
			return fmt.Errorf("timed out after %s: %w", timeout, err)
		}
		return err
	}

	notify := func(err error, next time.Duration) {
		// Retries are Info and tagged for the compact view. At Debug they reached the log file
		// and nothing else, so a check retrying for fifteen seconds looked exactly like a check
		// that had hung.
		log.InfoContext(ctx, fmt.Sprintf(
			"↻ %s attempt %d/%d failed: %s — retrying in %s",
			check.Name, stats.count, attempts, oneLine(err), next.Round(100*time.Millisecond),
		), dhlog.ShowInCompacted())
	}

	err := backoff.RetryNotify(operation, bo, notify)
	return detail, stats, err
}

// oneLine keeps a retry notice to a single line: the full text is in the final failure and in
// the log file, and a multi-line cause pushed through the live view scrolls the phase away.
func oneLine(err error) string {
	msg := strings.TrimSpace(err.Error())
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = strings.TrimSpace(msg[:idx]) + " …"
	}
	return msg
}

func summarize(results []Result, notRun int) string {
	counts := map[Status]int{}
	cached := 0
	for _, r := range results {
		switch r.Status {
		case StatusCached:
			counts[StatusPassed]++
			cached++
		default:
			counts[r.Status]++
		}
	}

	parts := []string{fmt.Sprintf("%d passed", counts[StatusPassed])}
	if cached > 0 {
		parts[0] = fmt.Sprintf("%d passed (%d cached)", counts[StatusPassed], cached)
	}
	for _, s := range []Status{StatusFailed, StatusBlocked, StatusSkipped, StatusNotApplicable} {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	if notRun > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", notRun, phaseStoppedSuffix))
	}
	return strings.Join(parts, ", ")
}

func (p *Preflight) cacheEnabled(check Check) bool {
	return p.cache != nil && !p.noCache && check.Cacheable
}

// cached reports a remembered pass and how old it is. The stored value is the RFC3339 time of
// the run that produced it, so the record can say what age of answer it is standing on; entries
// written before that format carry no time, and report a zero age.
func (p *Preflight) cached(ctx context.Context, check Check) (time.Duration, bool) {
	if !p.cacheEnabled(check) {
		return 0, false
	}
	key := p.cacheKey(check.Name)
	ok, err := p.cache.InCache(ctx, key)
	if err != nil || !ok {
		return 0, false
	}

	raw, err := p.cache.Load(ctx, key)
	if err != nil {
		return 0, true
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, true
	}
	return time.Since(at), true
}

func (p *Preflight) store(ctx context.Context, check Check) {
	if !p.cacheEnabled(check) {
		return
	}
	value := []byte(time.Now().Format(time.RFC3339))
	if err := p.cache.Save(ctx, p.cacheKey(check.Name), value); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"Cannot remember that %s passed: %v. The check will run again next time.",
			check.Name, err,
		))
	}
}

func (p *Preflight) cacheKey(name CheckName) string {
	if p.cacheSalt == "" {
		return fmt.Sprintf("preflight-%s", name)
	}
	return fmt.Sprintf("preflight-%s-%s", p.cacheSalt, name)
}
