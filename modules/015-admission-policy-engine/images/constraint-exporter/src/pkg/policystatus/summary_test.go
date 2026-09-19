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

package policystatus

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
)

func constraint(kind, policyKind, policyName string, total float64, violations ...*gatekeeper.Violation) gatekeeper.Constraint {
	return gatekeeper.Constraint{
		Meta: gatekeeper.ConstraintMeta{
			Kind:       kind,
			Name:       policyName,
			PolicyKind: policyKind,
			PolicyName: policyName,
		},
		Status: gatekeeper.ConstraintStatus{
			TotalViolations: total,
			Violations:      violations,
		},
	}
}

func violation(kind, namespace, name, action string) *gatekeeper.Violation {
	return &gatekeeper.Violation{
		Kind:              kind,
		Name:              name,
		Namespace:         namespace,
		Message:           "violation of " + name,
		EnforcementAction: action,
	}
}

func TestBuildGroupsViolationsByOwningPolicy(t *testing.T) {
	constraints := []gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 2,
			violation("Pod", "prod", "web", "deny"),
			violation("Pod", "stage", "api", "warn"),
		),
		constraint("D8HostNetwork", gatekeeper.SecurityPolicyKind, "app", 1,
			violation("Deployment", "prod", "gateway", "deny"),
		),
		constraint("D8RequiredLabels", gatekeeper.OperationPolicyKind, "labels", 1,
			violation("Pod", "prod", "web", "dryrun"),
		),
	}

	summaries := Build(constraints)
	require.Len(t, summaries, 2)

	app := summaries[Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}]
	assert.Equal(t, int64(3), app.Total)
	assert.Zero(t, app.Truncated)
	assert.Equal(t, map[string]int64{"deny": 2, "warn": 1}, app.ByEnforcement)
	assert.Equal(t, []NamespaceCount{{Namespace: "prod", Count: 2}, {Namespace: "stage", Count: 1}}, app.TopNamespaces)
	assert.Len(t, app.Sample, 3)

	labels := summaries[Owner{Kind: gatekeeper.OperationPolicyKind, Name: "labels"}]
	assert.Equal(t, int64(1), labels.Total)
	assert.Equal(t, map[string]int64{"dryrun": 1}, labels.ByEnforcement)
}

func TestBuildKeepsTotalExactWhenTheAuditTruncates(t *testing.T) {
	// The audit records at most --constraint-violations-limit violations per constraint, but still
	// reports how many there are. The summary has to carry both numbers.
	summaries := Build([]gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 250,
			violation("Pod", "prod", "web", "deny"),
		),
	})

	app := summaries[Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}]
	assert.Equal(t, int64(250), app.Total)
	assert.Equal(t, int64(249), app.Truncated)
	assert.Len(t, app.Sample, 1, "the sample covers only what the audit recorded")
}

func TestBuildReportsAPolicyWithoutViolations(t *testing.T) {
	// A policy whose violations are gone still needs a summary, otherwise the status would keep
	// reporting the violations of an earlier audit forever.
	summaries := Build([]gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 0),
	})

	app, ok := summaries[Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}]
	require.True(t, ok)
	assert.Zero(t, app.Total)
	assert.Nil(t, app.ByEnforcement)
	assert.Nil(t, app.TopNamespaces)
	assert.Nil(t, app.Sample)
}

func TestBuildSkipsAConstraintThatBelongsToNoPolicy(t *testing.T) {
	// deny-exec-heritage-pods belongs to the module itself and names no owner.
	summaries := Build([]gatekeeper.Constraint{
		{
			Meta:   gatekeeper.ConstraintMeta{Kind: "D8DenyExecHeritage", Name: "deny-exec-heritage-pods"},
			Status: gatekeeper.ConstraintStatus{TotalViolations: 5},
		},
	})

	assert.Empty(t, summaries)
}

func TestBuildBoundsTheLists(t *testing.T) {
	violations := make([]*gatekeeper.Violation, 0, 40)
	for i := 0; i < 40; i++ {
		violations = append(violations, violation("Pod", fmt.Sprintf("ns-%02d", i), fmt.Sprintf("pod-%02d", i), "deny"))
	}

	summaries := Build([]gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 40, violations...),
	})

	app := summaries[Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}]
	assert.Equal(t, int64(40), app.Total)
	assert.Len(t, app.TopNamespaces, maxTopNamespaces)
	assert.Len(t, app.Sample, maxSample)
}

func TestBuildIsStableAcrossRuns(t *testing.T) {
	// An unstable order would change the summary on every audit cycle and rewrite the status of a
	// policy whose violations did not change.
	violations := []*gatekeeper.Violation{
		violation("Pod", "b-ns", "second", "deny"),
		violation("Pod", "a-ns", "first", "warn"),
		violation("Pod", "a-ns", "third", "deny"),
	}

	first := Build([]gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 3, violations...),
	})

	for i := 0; i < 10; i++ {
		again := Build([]gatekeeper.Constraint{
			constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 3, violations...),
		})
		assert.Equal(t, first, again)
	}
}

func TestBuildIgnoresAnUnknownEnforcementAction(t *testing.T) {
	// The CRD prunes an unknown key of byEnforcement, so counting one would only lose the count.
	summaries := Build([]gatekeeper.Constraint{
		constraint("D8AllowedCapabilities", gatekeeper.SecurityPolicyKind, "app", 2,
			violation("Pod", "prod", "web", "deny"),
			violation("Pod", "prod", "api", "scream"),
		),
	})

	app := summaries[Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}]
	assert.Equal(t, int64(2), app.Total, "an unknown action still counts towards the total")
	assert.Equal(t, map[string]int64{"deny": 1}, app.ByEnforcement)
}
