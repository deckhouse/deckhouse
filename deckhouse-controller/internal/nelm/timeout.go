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

package nelm

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/werf/nelm/pkg/legacy/progrep"
)

const (
	// timeoutGrace keeps nelm's own deadline behind ours: both bound the same
	// apply, but only ours cancels with a cause naming what it waited for.
	timeoutGrace = time.Minute

	// maxNamedResources bounds how many resources a timeout cause names — the
	// text reaches a package condition message.
	maxNamedResources = 5
)

// withApplyDeadline bounds ctx by timeout, cancelling it with a cause that names
// the resources the apply never finished. A non-positive timeout leaves the
// apply unbounded. The returned function ends the deadline and must be called.
func withApplyDeadline(ctx context.Context, timeout time.Duration, progress *progressTracker) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	if timeout <= 0 {
		return ctx, func() { cancel(nil) }
	}

	timer := time.AfterFunc(timeout, func() { cancel(progress.timeoutCause(timeout)) })

	return ctx, func() {
		timer.Stop()
		cancel(nil)
	}
}

// backstopTimeout is the deadline nelm itself gets. A non-zero Timeout is what
// makes ReleaseInstall return context.Cause rather than its own unwind error,
// which is how our cause reaches the caller; the grace keeps nelm's deadline
// from firing first and replacing that cause with a bare "action timed out".
func backstopTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return 0
	}

	return timeout + timeoutGrace
}

// progressTracker keeps the last progress report nelm sent while applying a
// release, so an apply that runs out of time can name what it was waiting for.
type progressTracker struct {
	mu     sync.Mutex
	latest progrep.ProgressReport
}

func (t *progressTracker) record(report progrep.ProgressReport) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.latest = report
}

// timeoutCause is the cancellation cause for an apply that outlived timeout.
func (t *progressTracker) timeoutCause(timeout time.Duration) error {
	waiting := t.waitingFor()
	if len(waiting) == 0 {
		return fmt.Errorf("apply timed out after %s", timeout)
	}

	return fmt.Errorf("apply timed out after %s, waiting for %s", timeout, joinResources(waiting))
}

// waitingFor names the unfinished resources of the active stage as "Kind/Name".
// Operations already under way — progressing, or failed and being retried — are
// what an apply hangs on, so the queued ones are named only when there are none.
func (t *progressTracker) waitingFor() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	ops := activeStage(t.latest)

	started := resourcesByStatus(ops, progrep.OperationStatusProgressing, progrep.OperationStatusFailed)
	if len(started) > 0 {
		return started
	}

	return resourcesByStatus(ops, progrep.OperationStatusPending)
}

// activeStage returns the operations of the stage nelm is executing: the last
// stage report holding any, matching how the status service picks the stage.
func activeStage(report progrep.ProgressReport) []progrep.Operation {
	for i := len(report.StageReports) - 1; i >= 0; i-- {
		if ops := report.StageReports[i].Operations; len(ops) > 0 {
			return ops
		}
	}

	return nil
}

// resourcesByStatus renders the distinct resources of ops in one of statuses. A
// resource with several operations — an apply and a readiness track, say — is
// named once.
func resourcesByStatus(ops []progrep.Operation, statuses ...progrep.OperationStatus) []string {
	resources := make([]string, 0, len(ops))
	seen := make(map[string]struct{}, len(ops))

	for _, op := range ops {
		if !slices.Contains(statuses, op.Status) {
			continue
		}

		resource := op.Kind + "/" + op.Name
		if _, ok := seen[resource]; ok {
			continue
		}

		seen[resource] = struct{}{}
		resources = append(resources, resource)
	}

	return resources
}

// joinResources renders at most maxNamedResources resources and counts the rest.
func joinResources(resources []string) string {
	if len(resources) <= maxNamedResources {
		return strings.Join(resources, ", ")
	}

	return fmt.Sprintf("%s and %d more",
		strings.Join(resources[:maxNamedResources], ", "), len(resources)-maxNamedResources)
}
