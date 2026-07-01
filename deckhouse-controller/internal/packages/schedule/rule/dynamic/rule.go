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

// Package dynamic provides the module enablement intent rule: the single
// highest-precedence soft vote that folds the external enable/disable signals a
// module carries (explicit ModuleConfig intent and the deprecated global-hook
// dynamic enable). The two signals are resolved upstream into one tri-state by
// the getter (the global module's IsEnabled), so the rule itself is a thin
// adapter that turns that tri-state into a soft Enable/Disable vote.
package dynamic

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// reasonEnabled is the condition reason attached to an Enable vote.
// It matches the Kubernetes reason pattern:
// ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const reasonEnabled = "Enabled"

// reasonDisabled is the condition reason attached to a Disable vote.
const reasonDisabled = "Disabled"

// Getter reports a module's resolved enablement intent as a tri-state:
//   - *true  - the module is intended to be enabled;
//   - *false - the module is intended to be disabled;
//   - nil    - no opinion; resolution defers to the rest of the chain.
type Getter func(module string) *bool

// Rule is the module enablement intent rule. It is the highest-precedence soft
// vote: a non-nil getter result both enables and disables, overriding the floor
// and the bundle vote. It never vetoes — requirement gates still Forbid from any
// position, so an Enable cannot override an unmet requirement.
type Rule struct {
	getter Getter
	module string
}

// NewRule constructs a dynamic rule for the given module, resolving its intent
// through the getter at decision time.
func NewRule(getter Getter, module string) *Rule {
	return &Rule{
		getter: getter,
		module: module,
	}
}

// Decide returns a soft Enable when the module is intended enabled, a soft
// Disable when it is intended disabled, and Undefined when there is no opinion.
func (r *Rule) Decide() rule.Decision {
	enabled := r.getter(r.module)
	if enabled == nil {
		return rule.Decision{Kind: rule.Undefined}
	}

	if *enabled {
		return rule.Decision{
			Kind:   rule.Enable,
			Reason: reasonEnabled,
		}
	}

	return rule.Decision{
		Kind:   rule.Disable,
		Reason: reasonDisabled,
	}
}
