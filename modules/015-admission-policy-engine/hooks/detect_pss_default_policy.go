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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	apeRequirements "github.com/deckhouse/deckhouse/modules/015-admission-policy-engine/requirements"
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
	policy := input.ConfigValues.Get("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()
	explicit := policy != ""
	if !explicit {
		policy = deriveDefaultPolicy(ctx, input)
	}
	input.Values.Set("admissionPolicyEngine.podSecurityStandards.defaultPolicy", policy)
	input.MetricsCollector.Expire(defaultPolicyMetricGroup)
	input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_policy", policyCode(policy), map[string]string{}, metrics.WithGroup(defaultPolicyMetricGroup))

	// Privileged derived from the bootstrap version: the next minor release makes Baseline the unconditional default,
	// so the administrator has to set the default policy in the ModuleConfig, the alert asks for it and the update is blocked
	implicitPrivileged := !explicit && policy == "Privileged"
	if implicitPrivileged {
		input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_policy_not_set", 1, map[string]string{}, metrics.WithGroup(defaultPolicyMetricGroup))
	}
	requirements.SaveValue(apeRequirements.ImplicitPrivilegedKey, implicitPrivileged)
	return nil
}

// deriveDefaultPolicy derives the default policy from the version the cluster was bootstrapped with.
func deriveDefaultPolicy(_ context.Context, input *go_hook.HookInput) string {
	installDataSlice, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "install_data")
	if err != nil {
		input.Logger.Error("failed to unmarshal install_data snapshot, PSS default policy is set to Privileged, set podSecurityStandards.defaultPolicy in the ModuleConfig explicitly", log.Err(err))
		return "Privileged"
	}

	// no map found - an old cluster, bootstrapped before install-data was introduced in v1.55
	if len(installDataSlice) == 0 {
		input.Logger.Warn("install-data configmap isn't found, PSS default policy is set to Privileged, set podSecurityStandards.defaultPolicy in the ModuleConfig explicitly")
		return "Privileged"
	}

	deckhouseVersion := installDataSlice[0]

	// builds without a release tag (pull requests, main, release branches) write "dev",
	// such a build is always newer than v1.55
	if deckhouseVersion == "dev" {
		input.Logger.Info("PSS default policy is set to Baseline for a dev build", slog.String("version", deckhouseVersion))
		return "Baseline"
	}

	// no version field found or invalid semver - the bootstrap version is unknown
	if len(deckhouseVersion) == 0 || !semver.IsValid(deckhouseVersion) {
		input.Logger.Warn("deckhouseVersion isn't found or invalid, PSS default policy is set to Privileged, set podSecurityStandards.defaultPolicy in the ModuleConfig explicitly", slog.String("version", deckhouseVersion))
		return "Privileged"
	}

	// if deckhouse bootstrap release is lower than v1.55
	if semver.Compare(semver.MajorMinor(deckhouseVersion), milestone) < 0 {
		input.Logger.Warn("deckhouse bootstrap version is lower than v1.55, PSS default policy is set to Privileged, set podSecurityStandards.defaultPolicy in the ModuleConfig explicitly", slog.String("version", deckhouseVersion))
		return "Privileged"
	}

	return "Baseline"
}

func getVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "data", "version")
	if err != nil {
		return "", err
	}

	return version, nil
}
