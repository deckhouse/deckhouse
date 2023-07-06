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
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// The Order below matters for ensure_crds_istio.go, it needs globalVersion to deploy proper CRDs
	OnStartup:    &go_hook.OrderedConfig{Order: 5},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(revisionsDiscovery))

func revisionsDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	var globalVersion string
	var versionsToInstall = make([]string, 0)
	var unsupportedVersions = make([]string, 0)
	var supportedVersions = make([]string, 0)

	var supportedVersionsResult = input.Values.Get("istio.internal.versionMap").Map()
	for versionResult := range supportedVersionsResult {
		supportedVersions = append(supportedVersions, versionResult)
	}

	switch {
	case input.ConfigValues.Exists("istio.globalVersion"):
		// globalVersion is set in CM — use it
		globalVersion = input.ConfigValues.Get("istio.globalVersion").String()
	case input.Values.Exists("istio.internal.globalVersion"):
		// globalVersion was previously discovered — use it
		globalVersion = input.Values.Get("istio.internal.globalVersion").String()
	default:
		// maybe there is a global istiod Service with annotation?
		k8sClient, err := dc.GetK8sClient()
		if err != nil {
			return err
		}

		service, err := k8sClient.CoreV1().Services("d8-istio").Get(context.TODO(), "istiod", metav1.GetOptions{})
		if err == nil {
			// there is the global istiod Service — let's get the annotation
			if version, ok := service.GetAnnotations()["istio.deckhouse.io/global-version"]; ok {
				globalVersion = version
			} else {
				return fmt.Errorf("can't find istio.deckhouse.io/global-version annotation for istiod global Service d8-istio/istiod")
			}
		}
	}

	// couldn't discover globalVersion — let's use default value from openapi/config-values.yaml
	if globalVersion == "" {
		globalVersion = input.Values.Get("istio.globalVersion").String()
	}

	var additionalVersionsResult = input.ConfigValues.Get("istio.additionalVersions").Array()
	for _, versionResult := range additionalVersionsResult {
		if !lib.Contains(supportedVersions, versionResult.String()) {
			unsupportedVersions = append(unsupportedVersions, versionResult.String())
			continue
		}
		versionsToInstall = append(versionsToInstall, versionResult.String())
	}

	if !lib.Contains(supportedVersions, globalVersion) {
		if !lib.Contains(unsupportedVersions, globalVersion) {
			unsupportedVersions = append(unsupportedVersions, globalVersion)
		}
	} else {
		if !lib.Contains(versionsToInstall, globalVersion) {
			versionsToInstall = append(versionsToInstall, globalVersion)
		}
	}

	if len(unsupportedVersions) > 0 {
		sort.Strings(unsupportedVersions)
		return fmt.Errorf("unsupported versions: [%s]", strings.Join(unsupportedVersions, ","))
	}

	sort.Strings(versionsToInstall) // to guarantee same order

	input.Values.Set("istio.internal.globalVersion", globalVersion)
	input.Values.Set("istio.internal.versionsToInstall", versionsToInstall)

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())
	for _, ver := range versionsToInstall {
		fullVer, ok := versionMap[ver]
		if !ok {
			input.LogEntry.Warnf("Not found full version for version to install %s", ver)
			continue
		}

		input.MetricsCollector.Set(telemetry.WrapName("istio_control_plane_full_version"), 1.0, map[string]string{
			"full_version": fullVer.FullVersion,
		})
	}

	return nil
}
