/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package policystatus turns the violations the audit recorded in the constraints into the summary
// the module keeps in the status of a policy resource.
//
// The summary is deliberately small. The status of a policy shares an object with its spec, and
// every write of it stores a whole new revision of that object in etcd, so the section holds
// aggregated counters and a short sample rather than the full list of violations. The full list
// stays in the d8_gatekeeper_exporter_constraint_violations metric.
package policystatus

import (
	"sort"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
)

const (
	// maxTopNamespaces and maxSample bound the two lists of the summary. They keep the object small
	// enough to be rewritten every time the set of violations changes.
	maxTopNamespaces = 10
	maxSample        = 10
)

// Known enforcement actions. An action outside this set is left out of the counters, because the
// CRD prunes an unknown key of byEnforcement anyway.
var knownEnforcementActions = map[string]bool{
	"deny":   true,
	"warn":   true,
	"dryrun": true,
}

// Owner identifies the policy resource whose status holds a summary.
type Owner struct {
	Kind string
	Name string
}

// NamespaceCount reports how many violations a namespace holds.
type NamespaceCount struct {
	Namespace string `json:"namespace"`
	Count     int64  `json:"count"`
}

// SampleViolation describes one violation, for a preview in the interface.
type SampleViolation struct {
	Kind              string `json:"kind,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
	Name              string `json:"name,omitempty"`
	Rule              string `json:"rule,omitempty"`
	EnforcementAction string `json:"enforcementAction,omitempty"`
	Message           string `json:"message,omitempty"`
}

// Summary is the status.violations section of a policy resource.
//
// It carries no timestamp: the writer adds lastUpdateTime when it stores a summary that differs
// from the one already in the object.
type Summary struct {
	Total         int64             `json:"total"`
	Truncated     int64             `json:"truncated,omitempty"`
	ByEnforcement map[string]int64  `json:"byEnforcement,omitempty"`
	TopNamespaces []NamespaceCount  `json:"topNamespaces,omitempty"`
	Sample        []SampleViolation `json:"sample,omitempty"`
}

// Build groups the violations of the constraints by the policy that owns them.
//
// Every policy that has at least one constraint ends up in the result, so a policy whose violations
// are gone gets a summary with a zero total rather than keeping the previous one.
//
// Total is exact: it sums up the totalViolations the audit reported. The other counters and the two
// lists cover only the violations the audit recorded, which is why Truncated reports the difference.
func Build(constraints []gatekeeper.Constraint) map[Owner]Summary {
	type accumulator struct {
		summary    Summary
		namespaces map[string]int64
		violations []SampleViolation
	}

	acc := make(map[Owner]*accumulator)

	for _, c := range constraints {
		if c.Meta.PolicyName == "" || c.Meta.PolicyKind == "" {
			continue
		}

		owner := Owner{Kind: c.Meta.PolicyKind, Name: c.Meta.PolicyName}
		a, ok := acc[owner]
		if !ok {
			a = &accumulator{
				summary:    Summary{ByEnforcement: make(map[string]int64)},
				namespaces: make(map[string]int64),
			}
			acc[owner] = a
		}

		total := int64(c.Status.TotalViolations)
		a.summary.Total += total
		if hidden := total - int64(len(c.Status.Violations)); hidden > 0 {
			a.summary.Truncated += hidden
		}

		for _, v := range c.Status.Violations {
			if v == nil {
				continue
			}
			if knownEnforcementActions[v.EnforcementAction] {
				a.summary.ByEnforcement[v.EnforcementAction]++
			}
			a.namespaces[v.Namespace]++
			a.violations = append(a.violations, SampleViolation{
				Kind:              v.Kind,
				Namespace:         v.Namespace,
				Name:              v.Name,
				Rule:              c.Meta.Kind,
				EnforcementAction: v.EnforcementAction,
				Message:           v.Message,
			})
		}
	}

	result := make(map[Owner]Summary, len(acc))
	for owner, a := range acc {
		summary := a.summary
		if len(summary.ByEnforcement) == 0 {
			summary.ByEnforcement = nil
		}
		summary.TopNamespaces = topNamespaces(a.namespaces)
		summary.Sample = sampleViolations(a.violations)
		result[owner] = summary
	}

	return result
}

// topNamespaces returns the namespaces with the most violations, the largest first.
//
// The order is total, because an equal count is broken by the name of the namespace. A summary that
// reordered itself between two identical audits would be rewritten for nothing.
func topNamespaces(counts map[string]int64) []NamespaceCount {
	if len(counts) == 0 {
		return nil
	}

	list := make([]NamespaceCount, 0, len(counts))
	for ns, count := range counts {
		list = append(list, NamespaceCount{Namespace: ns, Count: count})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Namespace < list[j].Namespace
	})

	if len(list) > maxTopNamespaces {
		list = list[:maxTopNamespaces]
	}
	return list
}

// sampleViolations returns a stable slice of the violations, for the same reason as topNamespaces.
func sampleViolations(violations []SampleViolation) []SampleViolation {
	if len(violations) == 0 {
		return nil
	}

	sort.Slice(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		switch {
		case a.Namespace != b.Namespace:
			return a.Namespace < b.Namespace
		case a.Kind != b.Kind:
			return a.Kind < b.Kind
		case a.Name != b.Name:
			return a.Name < b.Name
		case a.Rule != b.Rule:
			return a.Rule < b.Rule
		default:
			return a.Message < b.Message
		}
	})

	if len(violations) > maxSample {
		violations = violations[:maxSample]
	}
	return violations
}
