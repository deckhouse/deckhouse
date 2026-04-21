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

package hooksync

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Condition reasons for hook sync failures.
const (
	ConditionReasonEventHookFailed status.ConditionReason = "EventHookFailed"
)

// newEventHookErr wraps an error when a sync hook execution fails.
// Carries only a reason; the calling task picks the condition via HandleError (HooksReady).
func newEventHookErr(err error) error {
	return &status.Error{
		Err:    err,
		Reason: ConditionReasonEventHookFailed,
	}
}
