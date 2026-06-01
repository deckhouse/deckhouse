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

// reasonNoneOfDependenciesViolated is returned when at least one member of a
// NoneOf group is installed and matches its forbidden constraint. Matches the
// Kubernetes condition reason pattern ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$.
const reasonNoneOfDependenciesViolated = "NoneOfDependenciesViolated"

// NoneOfGroup is a group of forbidden dependencies: no member may be installed
// in a way that matches its constraint. A nil constraint on a member forbids
// the module at any installed version; a non-nil constraint narrows the
// forbidden range, so versions outside the constraint remain acceptable.
type NoneOfGroup struct {
	Name    string
	Members map[string]*semver.Constraints
}

// NoneOfChecker evaluates one or more NoneOf groups against the current
// dependency graph. Each group is violated independently — for the package to
// pass, every group must have zero installed members that match their
// constraints. NoneOf groups are checker-only and add no edges to the
// topological graph; "must not be installed" expresses an admission predicate,
// not an ordering relation.
type NoneOfChecker struct {
	getter Getter
	groups []NoneOfGroup
}

// NewNoneOfChecker constructs a NoneOfChecker that resolves member versions
// through the given Getter (shared with the regular dependency.Checker and
// the AnyOfChecker).
func NewNoneOfChecker(getter Getter, groups []NoneOfGroup) *NoneOfChecker {
	return &NoneOfChecker{
		getter: getter,
		groups: groups,
	}
}

// Check returns Enabled when every group has zero violators. The first failing
// group short-circuits the result; the Reason is NoneOfDependenciesViolated
// and the Message names the group plus its actual offending members in sorted
// order so identical inputs produce identical messages across reconciles.
func (c *NoneOfChecker) Check() checker.Result {
	for _, group := range c.groups {
		violators := c.groupViolators(group)
		if len(violators) == 0 {
			continue
		}

		return checker.Result{
			Reason:  reasonNoneOfDependenciesViolated,
			Message: noneOfFailureMessage(group, violators),
		}
	}

	return checker.Result{Enabled: true}
}

// groupViolators returns the sorted names of members that are installed and
// match their forbidden constraint. An empty result means the group passes.
func (c *NoneOfChecker) groupViolators(group NoneOfGroup) []string {
	var violators []string

	for name, constraint := range group.Members {
		version := removePrereleaseAndMetadata(c.getter(name))
		if version == nil {
			continue
		}

		if constraint == nil || constraint.Check(version) {
			violators = append(violators, name)
		}
	}

	sort.Strings(violators)

	return violators
}

// noneOfFailureMessage formats a deterministic error for a violated group,
// naming only the members that actually triggered the failure (not the full
// group membership). Sorted to keep the message stable across reconciles and
// prevent spurious Kubernetes condition flapping.
func noneOfFailureMessage(group NoneOfGroup, violators []string) string {
	return fmt.Sprintf("noneOf group '%s' violated: forbidden modules installed: [%s]", group.Name, strings.Join(violators, ", "))
}
