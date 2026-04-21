// Copyright 2025 Flant JSC
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

package run

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Condition reasons for run-related failures.
const (
	ConditionReasonBeforeHelmHooksFailed status.ConditionReason = "BeforeHelmHooksFailed"
	ConditionReasonAfterHelmHooksFailed  status.ConditionReason = "AfterHelmHooksFailed"
)

// newBeforeHelmHookErr wraps an error when BeforeHelm hooks fail.
// Carries only a reason; the calling task picks the condition via HandleError (HooksReady).
func newBeforeHelmHookErr(err error) error {
	return &status.Error{
		Err:    err,
		Reason: ConditionReasonBeforeHelmHooksFailed,
	}
}

// newAfterHelmHookErr wraps an error when AfterHelm hooks fail.
// Carries only a reason; the calling task picks the condition via HandleError (HooksReady).
func newAfterHelmHookErr(err error) error {
	return &status.Error{
		Err:    err,
		Reason: ConditionReasonAfterHelmHooksFailed,
	}
}
