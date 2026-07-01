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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// reasonAnyOfDependenciesUnmet is returned when no member of an AnyOf group is
// installed and satisfies its constraint. Matches the Kubernetes condition
// reason pattern ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$.
const reasonAnyOfDependenciesUnmet = "AnyOfDependenciesUnmet"

// AnyOfGroup is an alternative-dependency group: at least one member must be
// installed and satisfy its constraint for the group to pass. A nil constraint
// on a member means "any installed version of this alternative is acceptable".
type AnyOfGroup struct {
	Name    string
	Members map[string]*semver.Constraints
}

// AnyOfRule evaluates one or more AnyOf groups against the current dependency
// graph. Each group is satisfied independently — for the package to pass, every
// group must have ≥1 installed member that meets its constraint. It is a gate:
// it returns Undefined or Forbid only.
//
// AnyOf groups add no edges to the topological graph, so fallback chains across
// packages (A any-of {B, C}; B any-of {A, D}) do not produce cycles.
type AnyOfRule struct {
	getter Getter
	groups []AnyOfGroup
}

// NewAnyOfRule constructs an AnyOfRule that resolves member versions through
// the given Getter (shared with the regular dependency.Rule).
func NewAnyOfRule(getter Getter, groups []AnyOfGroup) *AnyOfRule {
	return &AnyOfRule{
		getter: getter,
		groups: groups,
	}
}

// Decide returns Undefined when every group has ≥1 installed member that
// satisfies its constraint. The first failing group short-circuits to Forbid;
// the Reason is AnyOfDependenciesUnmet and the Message names the group plus its
// members in sorted order so identical inputs produce identical messages across
// reconciles.
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

// groupSatisfied reports whether at least one member of the group is installed
// and satisfies its constraint. A nil constraint accepts any installed version.
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

// failureMessage formats a deterministic error for an unsatisfied group. Map
// iteration order is unspecified, so member names are sorted to keep the
// message stable across evaluations of the same input — this prevents spurious
// status flapping when the failure is surfaced as a Kubernetes condition.
func failureMessage(group AnyOfGroup) string {
	names := make([]string, 0, len(group.Members))
	for name := range group.Members {
		names = append(names, name)
	}

	sort.Strings(names)

	return fmt.Sprintf("anyOf group '%s' unmet: one of [%s] must be installed and satisfy its constraint", group.Name, strings.Join(names, ", "))
}
