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

package lifecycle

import "context"

// CancelCause is a reason a package context was cancelled. It answers to
// context.Canceled, so generic cancellation checks keep working on the error a
// cancelled task returns while the text still names the trigger.
type CancelCause string

func (c CancelCause) Error() string { return string(c) }

func (c CancelCause) Is(target error) bool { return target == context.Canceled }

// Causes a cancelled task reports through context.Cause, naming what superseded it.
var (
	errUpdateStarted            = CancelCause("update started")
	errUpdateSuperseded         = CancelCause("update superseded")
	errReconciliationSuperseded = CancelCause("reconciliation superseded")
	errRemovalStarted           = CancelCause("removal started")
)
