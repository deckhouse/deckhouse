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

package dynamic

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// reasonDynamicallyEnabled is the condition reason attached to a dynamic Enable
// vote. It matches the Kubernetes reason pattern:
// ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const reasonDynamicallyEnabled = "DynamicallyEnabled"

// Getter reports the dynamic enabled state of a module as a tri-state:
//   - *true  - the module is dynamically enabled (e.g. by an enabled script or
//     another module's runtime decision);
//   - *false - the module is dynamically known not to be enabled;
//   - nil    - no dynamic state is recorded for the module.
type Getter func(module string) *bool

// Rule is an intent rule: it contributes a soft Enable vote when the module is
// dynamically enabled. Unlike the gate rules (version, dependency, condition) it
// never vetoes — it only ever turns a package on. A module that is not
// dynamically enabled (the getter returns *false or nil) yields Undefined, so
// the rule defers to the rest of the resolution chain rather than disabling.
type Rule struct {
	getter Getter
	module string
}

// NewRule constructs a dynamic rule for the given module, resolving its dynamic
// enabled state through the getter at decision time.
func NewRule(getter Getter, module string) *Rule {
	return &Rule{
		getter: getter,
		module: module,
	}
}

// Decide returns a soft Enable vote when the module is dynamically enabled;
// otherwise (the getter returns *false or nil) it returns Undefined and defers
// to other rules.
func (r *Rule) Decide() rule.Decision {
	if enabled := r.getter(r.module); enabled != nil && *enabled {
		return rule.Decision{
			Kind:   rule.Enable,
			Reason: reasonDynamicallyEnabled,
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}
