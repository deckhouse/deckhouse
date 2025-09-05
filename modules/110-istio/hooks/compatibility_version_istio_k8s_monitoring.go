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
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var (
	monitoringMetricsGroup = "k8s_version_compatibility"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        lib.Queue("monitoring"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10}, // The hook relies on operatorVersionsToInstall value discovered in discovery_operator_versions_to_install.go before.
}, versionCompatibilityMonitoringHook)

func versionCompatibilityMonitoringHook(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get("istio.internal.operatorVersionsToInstall").Exists() {
		return nil
	}
	if !input.Values.Get("global.discovery.kubernetesVersion").Exists() {
		return nil
	}
	if !input.Values.Get("istio.internal.istioToK8sCompatibilityMap").Exists() {
		return nil
	}
	compatibilityMap := make(map[string][]string)

	// Major.Minor
	istioVersions := input.Values.Get("istio.internal.operatorVersionsToInstall").Array()
	// Major.Minor.Patch
	k8sVersion := input.Values.Get("global.discovery.kubernetesVersion").String()
	k8sVersionSemver, err := semver.NewVersion(k8sVersion)
	if err != nil {
		return nil
	}
	k8sVersionMajorMinor := fmt.Sprintf("%d.%d", k8sVersionSemver.Major(), k8sVersionSemver.Minor())
	compatibilityMapStr := input.Values.Get("istio.internal.istioToK8sCompatibilityMap").String()
	_ = json.Unmarshal([]byte(compatibilityMapStr), &compatibilityMap)

	input.MetricsCollector.Expire(monitoringMetricsGroup)

OUTER:
	for _, istioVersion := range istioVersions {
		for _, k8sCompVersion := range compatibilityMap[istioVersion.String()] {
			if k8sVersionMajorMinor == k8sCompVersion {
				continue OUTER
			}
		}
		labels := map[string]string{
			"k8s_version":   k8sVersion,
			"istio_version": istioVersion.String(),
		}
		input.MetricsCollector.Set(telemetry.WrapName("istio_version_incompatible_with_k8s_version"), 1.0, labels, metrics.WithGroup(monitoringMetricsGroup))
	}

	return nil
}
