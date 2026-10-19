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

// Package dynamic is a module's enable intent rule. The getter folds the Module.spec.enabled intent and
// the deprecated global-hook enable signal into one tri-state; an enable is a soft Enable (gates
// and the enabled script can still forbid), a disable is a Forbid no other rule can undo.
package dynamic

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

const reasonEnabled = "Enabled"

const reasonDisabled = "Disabled"

// Getter reports a module's intent: *true, *false, or nil for no opinion.
type Getter func(module string) *bool

// Rule is the intent rule of one module.
type Rule struct {
	getter Getter
	module string
}

// NewRule constructs the intent rule of module.
func NewRule(getter Getter, module string) *Rule {
	return &Rule{
		getter: getter,
		module: module,
	}
}

// Decide returns Enable, Forbid or Undefined for the intent.
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
		Kind:   rule.Forbid,
		Reason: reasonDisabled,
	}
}
