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

package hooks

import (
	"context"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

const pssDefaultPolicyMetric = "d8_admission_policy_engine_pss_default_policy"

// The admission-policy-engine Grafana dashboard reads this metric to show the policy that applies to
// namespaces without the `security.deckhouse.io/pod-policy` label.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, exportDefaultPolicyMetric)

func policyCode(name string) float64 {
	switch strings.ToLower(name) {
	case "restricted":
		return 3
	case "baseline":
		return 2
	case "privileged":
		return 1
	default:
		return 0
	}
}

func exportDefaultPolicyMetric(_ context.Context, input *go_hook.HookInput) error {
	policy := input.Values.Get("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()
	// The OpenAPI schema sets Baseline as the default; the templates fall back to it in the same way.
	if policy == "" {
		policy = "Baseline"
	}
	input.MetricsCollector.Expire(pssDefaultPolicyMetric)
	input.MetricsCollector.Set(pssDefaultPolicyMetric, policyCode(policy), map[string]string{}, metrics.WithGroup(pssDefaultPolicyMetric))
	return nil
}
