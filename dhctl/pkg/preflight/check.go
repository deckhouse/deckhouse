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
	"fmt"
	"regexp"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type CheckName string

func (n CheckName) String() string {
	return string(n)
}

// examles: dvp-kubeconfig, dhctl-edition
var checkNamePattern = regexp.MustCompile("^[a-z][a-z-]*$")

func (n CheckName) Validate() error {
	if checkNamePattern.MatchString(n.String()) {
		return nil
	}
	return fmt.Errorf("invalid preflight check name %q: must match %s", n, checkNamePattern.String())
}

// RunFunc is the body of a check. On success it may return the value-bearing line for the ✓
// record — "authenticated to https://registry.deckhouse.ru as \"license-token\"", the CIDRs it
// compared, the host it logged in to. An empty string falls back to Check.Description, which is
// an assertion rather than an observation and so says nothing about this cluster.
//
// On failure it returns a *Failure where it can name the field, host or command it looked at,
// and a bare error where it cannot. NotApplicable ends it with the outcome that is not a pass.
type RunFunc func(ctx context.Context) (string, error)

// Detailless adapts a check body that reports nothing beyond pass or fail. Checks migrate off it
// one at a time, as each learns what to say about the cluster it just looked at.
func Detailless(run func(ctx context.Context) error) RunFunc {
	return func(ctx context.Context) (string, error) {
		return "", run(ctx)
	}
}

type Check struct {
	Name CheckName
	// Description is the assertion that holds when the check passes, phrased so it reads under
	// both a ✓ and a ✗: "sudo is installed and allowed for user", not "check sudo".
	Description string
	Phase       Phase
	Run         RunFunc
	Retry       RetryPolicy
	// Timeout bounds a single attempt. Zero means DefaultPreflightCheckTimeout: without one, a
	// registry that accepts the connection and never answers hangs the whole bootstrap.
	Timeout time.Duration
	// Cacheable marks a check whose verdict is a pure function of the configuration, which is
	// what the cache key is built from. Anything that looks at a node is not: the node can be
	// replaced behind the same address between two runs, and the cached ✓ would be about a
	// machine that no longer exists.
	Cacheable bool
	// CannotBeSkipped marks a check that guards an assumption the rest of dhctl is written
	// against, so --preflight-skip-check must refuse to turn it off rather than let the run
	// proceed into a state it cannot handle.
	CannotBeSkipped bool
	// CannotBeSkippedReason is why, in the reader's terms. The report prints it in place of the
	// skip flag it cannot offer: "this check cannot be skipped" answers a question the reader
	// did not ask and leaves the one they did.
	CannotBeSkippedReason string
	// DependsOn names the checks this one needs to have passed. When one of them fails, this
	// check is reported blocked instead of being run: it would fail for the same reason and
	// bury the one error that matters under a pile of copies.
	DependsOn []CheckName
	// StopsPhaseOnFailure marks a check the rest of the phase stands on. When it fails the
	// runner stops there: not because the remaining checks would be wrong, but because they
	// cannot be asked at all, and because the failure that matters should be the last thing on
	// the screen rather than the first line above a page of records saying nothing happened.
	//
	// It is for the critical path only — the SSH connection every node check is made over. A
	// check whose failure merely makes another one pointless (sudo-installed before
	// sudo-allowed) uses DependsOn, which reports the dependent as blocked and carries on with
	// everything else.
	StopsPhaseOnFailure bool

	Disabled bool
	// DisabledReason says who turned the check off and why. Empty means the user did, with a
	// flag; DisableCheckWithReason fills it in when dhctl itself does.
	DisabledReason string
}

func (c *Check) Disable() {
	c.Disabled = true
}

// After records that this check needs the named ones to have passed. It returns the check by
// value so it can be written inline in a suite.
func (c Check) After(names ...CheckName) Check {
	c.DependsOn = append(append([]CheckName(nil), c.DependsOn...), names...)
	return c
}

type RetryPolicy struct {
	Attempts int
	Options  []backoff.ExponentialBackOffOpts
}

// NoRetry is for a check whose verdict cannot change between two attempts: it reads the
// configuration, or it asks a question the node answers the same way every time (a sudoers rule,
// a user that exists, a port that is bound).
var NoRetry = RetryPolicy{Attempts: 1}

// NetworkRetry is for a check that crosses a network, where the first answer may be no answer at
// all. Three attempts, waiting 1s then 2s — 3 seconds of waiting in the worst case, against the
// 15 seconds the old five-attempt policy spent retrying answers like HTTP 401 that were never
// going to change. A deterministic outcome inside a NetworkRetry check returns Permanent(err)
// and stops the retrying then and there.
var NetworkRetry = RetryPolicy{
	Attempts: 3,
	Options: []backoff.ExponentialBackOffOpts{
		backoff.WithInitialInterval(time.Second),
		backoff.WithMultiplier(2),
		backoff.WithMaxElapsedTime(0),
	},
}

// DefaultRetryPolicy is NetworkRetry under the name the existing checks reference.
var DefaultRetryPolicy = NetworkRetry

// Permanent marks err as a verdict no further attempt can change — an HTTP 401, a missing
// sudoers rule, a port already bound, an edition that does not match. The runner reports it
// immediately and says it was not retried; errors.Is and errors.As still see err itself.
func Permanent(err error) error {
	return backoff.Permanent(err)
}
