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

package metrics

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCollectorAggregatesPerKind(t *testing.T) {
	t.Parallel()
	c := New()
	c.Observe(KindClusterAuthorizationRule, "dev", Observation{Desired: 3, Actual: 3})
	c.Observe(KindClusterAuthorizationRule, "ops", Observation{Desired: 2, Actual: 1, Drift: Drift{Missing: 1}})
	c.Observe(KindAuthorizationRule, "team/dev", Observation{Desired: 2, Actual: 3, Drift: Drift{Extra: 1}})

	expected := `
# HELP d8_user_authz_authorization_rules Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).
# TYPE d8_user_authz_authorization_rules gauge
d8_user_authz_authorization_rules{kind="AuthorizationRule"} 1
d8_user_authz_authorization_rules{kind="ClusterAuthorizationRule"} 2
# HELP d8_user_authz_bindings_actual Bindings of the reconciled objects of a kind after their last reconcile; equals desired once the controller converged.
# TYPE d8_user_authz_bindings_actual gauge
d8_user_authz_bindings_actual{kind="AuthorizationRule"} 3
d8_user_authz_bindings_actual{kind="ClusterAuthorizationRule"} 4
# HELP d8_user_authz_bindings_desired Bindings the reconciled objects of a kind must have.
# TYPE d8_user_authz_bindings_desired gauge
d8_user_authz_bindings_desired{kind="AuthorizationRule"} 2
d8_user_authz_bindings_desired{kind="ClusterAuthorizationRule"} 5
# HELP d8_user_authz_bindings_drift Bindings still not in the desired state after the last reconcile, by reason (missing, extra, changed); above zero only while the controller fails to converge.
# TYPE d8_user_authz_bindings_drift gauge
d8_user_authz_bindings_drift{kind="AuthorizationRule",reason="changed"} 0
d8_user_authz_bindings_drift{kind="AuthorizationRule",reason="extra"} 1
d8_user_authz_bindings_drift{kind="AuthorizationRule",reason="missing"} 0
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="changed"} 0
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="extra"} 0
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="missing"} 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"d8_user_authz_authorization_rules", "d8_user_authz_bindings_desired", "d8_user_authz_bindings_actual", "d8_user_authz_bindings_drift"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorInvalidRulesAndForget(t *testing.T) {
	t.Parallel()
	c := New()
	c.Observe(KindClusterAuthorizationRule, "bad", Observation{})
	c.SetInvalid(KindClusterAuthorizationRule, "bad", "bad", "", "InvalidSpec")
	c.Observe(KindAuthorizationRule, "team/bad", Observation{})
	c.SetInvalid(KindAuthorizationRule, "team/bad", "bad", "team", "ApplyError")

	expected := `
# HELP d8_user_authz_authorization_rule_invalid Rules whose bindings cannot be applied (1 per rule, at most MaxInvalidSeries rules), with the reason of the Ready=False condition.
# TYPE d8_user_authz_authorization_rule_invalid gauge
d8_user_authz_authorization_rule_invalid{kind="AuthorizationRule",name="bad",reason="ApplyError",rule_namespace="team"} 1
d8_user_authz_authorization_rule_invalid{kind="ClusterAuthorizationRule",name="bad",reason="InvalidSpec",rule_namespace=""} 1
# HELP d8_user_authz_authorization_rules_invalid Number of rules whose bindings cannot be applied, by kind and reason of the Ready=False condition.
# TYPE d8_user_authz_authorization_rules_invalid gauge
d8_user_authz_authorization_rules_invalid{kind="AuthorizationRule",reason="ApplyError"} 1
d8_user_authz_authorization_rules_invalid{kind="ClusterAuthorizationRule",reason="InvalidSpec"} 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "d8_user_authz_authorization_rule_invalid", "d8_user_authz_authorization_rules_invalid"); err != nil {
		t.Fatal(err)
	}

	c.ClearInvalid(KindAuthorizationRule, "team/bad")
	c.Forget(KindClusterAuthorizationRule, "bad")
	if n := testutil.CollectAndCount(c, "d8_user_authz_authorization_rule_invalid"); n != 0 {
		t.Errorf("invalid series after clear/forget = %d, want 0", n)
	}
	expectedRules := `
# HELP d8_user_authz_authorization_rules Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).
# TYPE d8_user_authz_authorization_rules gauge
d8_user_authz_authorization_rules{kind="AuthorizationRule"} 1
d8_user_authz_authorization_rules{kind="ClusterAuthorizationRule"} 0
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expectedRules), "d8_user_authz_authorization_rules"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorApplyCounterAndRegistration(t *testing.T) {
	t.Parallel()
	c := New()
	reg := prometheus.NewPedanticRegistry()
	if err := c.Register(reg); err != nil {
		t.Fatal(err)
	}
	if err := c.Register(reg); err != nil {
		t.Fatalf("second registration must be tolerated: %v", err)
	}

	c.RecordApply(KindClusterAuthorizationRule, OpCreate, nil)
	c.RecordApply(KindClusterAuthorizationRule, OpCreate, nil)
	c.RecordApply(KindClusterAuthorizationRule, OpDelete, errors.New("boom"))

	expected := `
# HELP d8_user_authz_bindings_apply_total Write operations on bindings issued by the controller, by kind of the reconciled object, operation and result.
# TYPE d8_user_authz_bindings_apply_total counter
d8_user_authz_bindings_apply_total{kind="ClusterAuthorizationRule",op="create",result="success"} 2
d8_user_authz_bindings_apply_total{kind="ClusterAuthorizationRule",op="delete",result="error"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "d8_user_authz_bindings_apply_total"); err != nil {
		t.Fatal(err)
	}
}

// A cluster-wide write failure marks every rule invalid at once: the named series are capped, the
// aggregate stays complete.
func TestCollectorCapsInvalidSeries(t *testing.T) {
	t.Parallel()
	c := New()
	for i := range MaxInvalidSeries + 25 {
		key := fmt.Sprintf("rule-%03d", i)
		c.SetInvalid(KindClusterAuthorizationRule, key, key, "", "ApplyError")
	}

	if n := testutil.CollectAndCount(c, "d8_user_authz_authorization_rule_invalid"); n != MaxInvalidSeries {
		t.Errorf("named invalid series = %d, want %d", n, MaxInvalidSeries)
	}
	expected := fmt.Sprintf(`
# HELP d8_user_authz_authorization_rules_invalid Number of rules whose bindings cannot be applied, by kind and reason of the Ready=False condition.
# TYPE d8_user_authz_authorization_rules_invalid gauge
d8_user_authz_authorization_rules_invalid{kind="ClusterAuthorizationRule",reason="ApplyError"} %d
`, MaxInvalidSeries+25)
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "d8_user_authz_authorization_rules_invalid"); err != nil {
		t.Fatal(err)
	}
}
