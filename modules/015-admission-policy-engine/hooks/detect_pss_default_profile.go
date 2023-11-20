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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const milestone = "v1.55"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "install_data",
			ApiVersion:                   "v1",
			Kind:                         "ConfigMap",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
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
}, setDefaultProfile)

func profileCode(action string) float64 {
	switch strings.ToLower(action) {
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

func setDefaultProfile(input *go_hook.HookInput) error {
	profile := getDefaultProfile(input)
	input.Values.Set("admissionPolicyEngine.podSecurityStandards.defaultProfile", profile)
	input.MetricsCollector.Expire("d8_admission_policy_engine_pss_default_profile")
	input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_profile", profileCode(profile), map[string]string{}, metrics.WithGroup("d8_admission_policy_engine_pss_default_profile"))
	return nil
}

func getDefaultProfile(input *go_hook.HookInput) string {
	// default profile is set explicitly - nothing to do here
	if profile := input.ConfigValues.Get("admissionPolicyEngine.podSecurityStandards.defaultProfile").String(); profile != "" {
		return profile
	}

	// no map found - an old cluster
	if len(input.Snapshots["install_data"]) == 0 {
		return "Privileged"
	}

	deckhouseVersion := input.Snapshots["install_data"][0].(string)

	// no version field found or invalid semver - something went wrong
	if len(deckhouseVersion) == 0 || !semver.IsValid(deckhouseVersion) {
		input.LogEntry.Warnf("deckhouseVersion isn't found or invalid: %s", deckhouseVersion)
		return "Privileged"
	}

	// if deckhouse bootstrap release >= v1.55
	if semver.Compare(semver.MajorMinor(deckhouseVersion), milestone) >= 0 {
		input.LogEntry.Infof("PSS default profile for %v is set to baseline", deckhouseVersion)
		return "Baseline"
	}

	return "Privileged"
}

func getVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "data", "version")
	if err != nil {
		return "", err
	}

	return version, nil
}
