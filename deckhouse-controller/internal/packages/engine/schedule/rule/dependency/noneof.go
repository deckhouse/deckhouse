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

// reasonNoneOfDependenciesViolated: a member of a group is enabled at a forbidden version.
const reasonNoneOfDependenciesViolated = "NoneOfDependenciesViolated"

// NoneOfGroup passes when no member matches its forbidden constraint (nil: any version).
type NoneOfGroup struct {
	Name    string
	Members map[string]*semver.Constraints
}

// NoneOfRule is a gate: every group must pass. It adds no ordering edge: "must not be installed"
// is a predicate, not an order.
type NoneOfRule struct {
	getter Getter
	groups []NoneOfGroup
}

// NewNoneOfRule constructs a NoneOfRule.
func NewNoneOfRule(getter Getter, groups []NoneOfGroup) *NoneOfRule {
	return &NoneOfRule{
		getter: getter,
		groups: groups,
	}
}

// Decide forbids on the first violated group, naming it and its violators.
func (r *NoneOfRule) Decide() rule.Decision {
	for _, group := range r.groups {
		violators := r.groupViolators(group)
		if len(violators) == 0 {
			continue
		}

		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  reasonNoneOfDependenciesViolated,
			Message: noneOfFailureMessage(group, violators),
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}

func (r *NoneOfRule) groupViolators(group NoneOfGroup) []string {
	var violators []string

	for name, constraint := range group.Members {
		version := removePrereleaseAndMetadata(r.getter(name))
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

func noneOfFailureMessage(group NoneOfGroup, violators []string) string {
	return fmt.Sprintf("noneOf group '%s' violated: forbidden modules installed: [%s]", group.Name, strings.Join(violators, ", "))
}
