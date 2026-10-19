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

package dependency

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

// reasonAnyOfDependenciesUnmet: no member of a group is enabled at a satisfying version.
const reasonAnyOfDependenciesUnmet = "AnyOfDependenciesUnmet"

// AnyOfGroup passes when one member satisfies its constraint (nil: any version).
type AnyOfGroup struct {
	Name    string
	Members map[string]*semver.Constraints
}

// AnyOfRule is a gate: every group must pass. The scheduler orders members only by soft edges,
// dropped where they close a cycle, so fallback chains (A any-of {B, C}; B any-of {A, D}) stay
// legal.
type AnyOfRule struct {
	getter Getter
	groups []AnyOfGroup
}

// NewAnyOfRule constructs an AnyOfRule.
func NewAnyOfRule(getter Getter, groups []AnyOfGroup) *AnyOfRule {
	return &AnyOfRule{
		getter: getter,
		groups: groups,
	}
}

// Decide forbids on the first failing group, naming it and its members.
func (r *AnyOfRule) Decide() rule.Decision {
	for _, group := range r.groups {
		if r.groupSatisfied(group) {
			continue
		}

		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  reasonAnyOfDependenciesUnmet,
			Message: failureMessage(group),
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}

func (r *AnyOfRule) groupSatisfied(group AnyOfGroup) bool {
	for name, constraint := range group.Members {
		version := removePrereleaseAndMetadata(r.getter(name))
		if version == nil {
			continue
		}

		if constraint == nil || constraint.Check(version) {
			return true
		}
	}

	return false
}

// failureMessage names the group and its members, sorted so the condition does not flap.
func failureMessage(group AnyOfGroup) string {
	names := make([]string, 0, len(group.Members))
	for name := range group.Members {
		names = append(names, name)
	}

	sort.Strings(names)

	return fmt.Sprintf("anyOf group '%s' unmet: one of [%s] must be installed and satisfy its constraint", group.Name, strings.Join(names, ", "))
}
