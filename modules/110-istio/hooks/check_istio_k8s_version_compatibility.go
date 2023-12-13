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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "main",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
}, checkIstioK8sVersionCompatibility)

func checkIstioK8sVersionCompatibility(input *go_hook.HookInput) error {
	compatibilityMap := make(map[string][]string)

	// Major.Minor
	istioVersions := input.Values.Get("istio.internal.operatorVersionsToInstall").Array()
	// Major.Minor.Patch
	k8sVersion := input.Values.Get("global.discovery.kubernetesVersion").String()
	k8sVersionSemver, err := semver.NewVersion(k8sVersion)
	if err != nil {
		return err
	}
	k8sVersionMajorMinor := fmt.Sprintf("%d.%d", k8sVersionSemver.Major(), k8sVersionSemver.Minor())
	compatibilityMapStr := input.Values.Get("istio.internal.istioToK8sCompatibilityMap").String()
	_ = json.Unmarshal([]byte(compatibilityMapStr), &compatibilityMap)

	OUTER:
	for _, istioVersion := range istioVersions {
		for _, k8sCompVersion := range compatibilityMap[istioVersion.String()] {
			if k8sCompVersion == k8sVersionMajorMinor {
				continue OUTER
			}
		}
		return fmt.Errorf("istio version '%s' is incompatible with k8s version '%s'", istioVersion, k8sVersion)
	}

	return nil
}
