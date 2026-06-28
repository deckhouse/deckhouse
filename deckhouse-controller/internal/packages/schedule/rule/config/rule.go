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

package config

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// reasonEnabledByConfig is the condition reason attached to a config Enable vote.
// It matches the Kubernetes reason pattern:
// ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const reasonEnabledByConfig = "EnabledByModuleConfig"

// reasonDisabledByConfig is the condition reason attached to a config Disable vote.
const reasonDisabledByConfig = "DisabledByModuleConfig"

// Getter reports the user's explicit enabled intent for a module, as carried by
// the ModuleConfig `enabled` field, as a tri-state:
//   - *true  - the user explicitly enabled the module;
//   - *false - the user explicitly disabled the module;
//   - nil    - no ModuleConfig opinion; resolution defers to other rules.
type Getter func(module string) *bool

// Rule is the highest-precedence intent rule: it carries explicit user intent
// from a ModuleConfig. Unlike the dynamic rule it can vote both ways — a soft
// Enable when the user turned the module on, a soft Disable when they turned it
// off — so a user can both enable a module the bundle ignores and disable one
// the bundle turns on. It never vetoes: requirement gates still Forbid from any
// position, so an explicit Enable cannot override an unmet requirement.
type Rule struct {
	getter Getter
	module string
}

// NewRule constructs a config rule for the given module, resolving its enabled
// intent through the getter at decision time.
func NewRule(getter Getter, module string) *Rule {
	return &Rule{
		getter: getter,
		module: module,
	}
}

// Decide returns a soft Enable when the user enabled the module, a soft Disable
// when they disabled it, and Undefined when no ModuleConfig opinion is recorded.
func (r *Rule) Decide() rule.Decision {
	enabled := r.getter(r.module)
	if enabled == nil {
		return rule.Decision{Kind: rule.Undefined}
	}

	if *enabled {
		return rule.Decision{
			Kind:   rule.Enable,
			Reason: reasonEnabledByConfig,
		}
	}

	return rule.Decision{
		Kind:   rule.Disable,
		Reason: reasonDisabledByConfig,
	}
}
