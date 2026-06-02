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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
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

// AnyOfChecker evaluates one or more AnyOf groups against the current
// dependency graph. Each group is satisfied independently — for the package to
// pass, every group must have ≥1 installed member that meets its constraint.
//
// AnyOf groups are checker-only and add no edges to the topological graph, so
// fallback chains across packages (A any-of {B, C}; B any-of {A, D}) do not
// produce cycles.
type AnyOfChecker struct {
	getter Getter
	groups []AnyOfGroup
}

// NewAnyOfChecker constructs an AnyOfChecker that resolves member versions
// through the given Getter (shared with the regular dependency.Checker).
func NewAnyOfChecker(getter Getter, groups []AnyOfGroup) *AnyOfChecker {
	return &AnyOfChecker{
		getter: getter,
		groups: groups,
	}
}

// Check returns Enabled when every group has ≥1 installed member that
// satisfies its constraint. The first failing group short-circuits the
// result; the Reason is AnyOfDependenciesUnmet and the Message names the
// group plus its members in sorted order so identical inputs produce
// identical messages across reconciles.
func (c *AnyOfChecker) Check() checker.Result {
	for _, group := range c.groups {
		if c.groupSatisfied(group) {
			continue
		}

		return checker.Result{
			Reason:  reasonAnyOfDependenciesUnmet,
			Message: failureMessage(group),
		}
	}

	return checker.Result{Enabled: true}
}

// groupSatisfied reports whether at least one member of the group is installed
// and satisfies its constraint. A nil constraint accepts any installed version.
func (c *AnyOfChecker) groupSatisfied(group AnyOfGroup) bool {
	for name, constraint := range group.Members {
		version := removePrereleaseAndMetadata(c.getter(name))
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
