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

package enable

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Condition reasons for startup-related failures.
const (
	ConditionReasonStartupHooksFailed status.ConditionReason = "StartupHookFailed"
	ConditionReasonInitHooksFailed    status.ConditionReason = "InitHooksFailed"
)

// newStartupHookErr wraps an error when OnStartup hooks fail.
// Sets HooksProcessed and ReadyInRuntime to False.
func newStartupHookErr(err error) error {
	return &status.Error{
		Err:    err,
		Reason: ConditionReasonStartupHooksFailed,
	}
}

// newInitHooksErr wraps an error when hook initialization fails.
// Sets ReadyInRuntime to False - package cannot proceed without hooks.
func newInitHooksErr(err error) error {
	return &status.Error{
		Err:    err,
		Reason: ConditionReasonInitHooksFailed,
	}
}
