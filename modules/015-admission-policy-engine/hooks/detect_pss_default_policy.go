/*
Copyright 2023 Flant JSC

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
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	milestone = "v1.55"

	defaultPolicyMetricGroup = "d8_admission_policy_engine_pss_default_policy"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "install_data",
			ApiVersion:                   "v1",
			Kind:                         "ConfigMap",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"install-data"},
			},
			FilterFunc: getVersion,
		},
	},
}, setDefaultPolicy)

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

func setDefaultPolicy(ctx context.Context, input *go_hook.HookInput) error {
	policy, mustBeSetExplicitly := getDefaultPolicy(ctx, input)
	input.Values.Set("admissionPolicyEngine.podSecurityStandards.defaultPolicy", policy)
	input.MetricsCollector.Expire(defaultPolicyMetricGroup)
	input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_policy", policyCode(policy), map[string]string{}, metrics.WithGroup(defaultPolicyMetricGroup))
	// the bootstrap version is unknown, so the administrator has to choose the default policy in the ModuleConfig
	if mustBeSetExplicitly {
		input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_policy_not_set", 1, map[string]string{}, metrics.WithGroup(defaultPolicyMetricGroup))
	}
	return nil
}

// getDefaultPolicy returns the default policy and whether it must be set explicitly in the ModuleConfig.
func getDefaultPolicy(_ context.Context, input *go_hook.HookInput) (string, bool) {
	// default policy is set explicitly - nothing to do here
	if policy := input.ConfigValues.Get("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String(); policy != "" {
		return policy, false
	}

	installDataSlice, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "install_data")
	if err != nil {
		input.Logger.Error("failed to unmarshal install_data snapshot", log.Err(err))
		return "Baseline", false
	}

	// no map found - an old cluster, bootstrapped before install-data was introduced in v1.55
	if len(installDataSlice) == 0 {
		input.Logger.Warn("install-data configmap isn't found, PSS default policy is set to privileged, set podSecurityStandards.defaultPolicy in the ModuleConfig explicitly")
		return "Privileged", true
	}

	deckhouseVersion := installDataSlice[0]

	// no version field found or invalid semver - something went wrong
	if len(deckhouseVersion) == 0 || !semver.IsValid(deckhouseVersion) {
		input.Logger.Warn("deckhouseVersion isn't found or invalid", slog.String("version", deckhouseVersion))
		return "Baseline", false
	}

	// if deckhouse bootstrap release < v1.55
	if semver.Compare(semver.MajorMinor(deckhouseVersion), milestone) < 0 {
		input.Logger.Info("PSS default policy is set to privileged", slog.String("version", deckhouseVersion))
		return "Privileged", false
	}

	return "Baseline", false
}

func getVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "data", "version")
	if err != nil {
		return "", err
	}

	return version, nil
}
