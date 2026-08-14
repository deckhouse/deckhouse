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

package providercheck

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type dexProviderCheckResult struct {
	checks                 []DexProviderCheckStepStatus
	acknowledgedWarnings   map[string]bool
	acknowledgeAllWarnings bool
	now                    time.Time
}

func (r *dexProviderCheckResult) succeed(name, format string, args ...any) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  stepSucceeded,
		Message: fmt.Sprintf(format, args...),
	})
}

// warn records a Warning step. The overall phase stays Succeeded. Steps listed
// in acknowledged-warnings (or "*") are recorded as Succeeded.
func (r *dexProviderCheckResult) warn(name, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if r.acknowledgeAllWarnings || r.acknowledgedWarnings[name] {
		r.checks = append(r.checks, DexProviderCheckStepStatus{
			Name:    name,
			Status:  stepSucceeded,
			Message: message + " (acknowledged via annotation)",
		})
		return
	}
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  stepWarning,
		Message: message,
	})
}

func (r *dexProviderCheckResult) fail(name, format string, args ...any) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  stepFailed,
		Message: fmt.Sprintf(format, args...),
	})
}

// failUnreachable records a Failed step. Network errors get a closed-contour hint:
// Dex would fail the same way, so this is not a false negative of the checker.
func (r *dexProviderCheckResult) failUnreachable(name string, err error, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if isNetworkUnreachable(err) {
		message += unreachableHint
	}
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  stepFailed,
		Message: message,
	})
}

func (r *dexProviderCheckResult) skip(name, message string) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  stepSkipped,
		Message: message,
	})
}

func (r *dexProviderCheckResult) status(observedGeneration int64) DexProviderCheckStatus {
	phase := DexProviderCheckPhaseSucceeded
	message := "connectivity check passed"
	for _, check := range r.checks {
		if check.Status == stepFailed {
			phase = DexProviderCheckPhaseFailed
			message = check.Message
			break
		}
	}

	completedAt := metav1.NewTime(r.now)
	return DexProviderCheckStatus{
		Phase:                         phase,
		Message:                       message,
		ObservedDexProviderGeneration: observedGeneration,
		Checks:                        r.checks,
		CompletedAt:                   &completedAt,
	}
}

// parseAcknowledgedWarnings reads the acknowledged-warnings annotation into a
// lookup set. A "*" entry acknowledges every warning.
func parseAcknowledgedWarnings(annotations map[string]string) (bool, map[string]bool) {
	raw, ok := annotations[acknowledgedWarningsAnnotation]
	if !ok {
		return false, nil
	}
	set := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		switch item {
		case "":
			continue
		case "*":
			return true, nil
		default:
			set[item] = true
		}
	}
	return false, set
}

func canonicalCheckName(providerName string) string {
	return providerName
}

func checkCompleted(check *DexProviderCheck) bool {
	return check.Status.Phase == DexProviderCheckPhaseSucceeded || check.Status.Phase == DexProviderCheckPhaseFailed
}

func checkUpToDate(check *DexProviderCheck, providerGeneration int64, now time.Time) bool {
	if check.Status.CompletedAt == nil {
		return false
	}
	if check.Status.ObservedDexProviderGeneration != providerGeneration {
		return false
	}
	return now.Sub(check.Status.CompletedAt.Time) < recheckInterval
}

func requeueRemaining(completedAt time.Time, now time.Time) time.Duration {
	remaining := recheckInterval - now.Sub(completedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}
