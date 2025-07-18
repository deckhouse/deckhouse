// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

var osImageUbuntuRegex = regexp.MustCompile(`^Ubuntu ([0-9]{2}\.[0-9]{2}\.[0-9]+)( )?(LTS)?$`)
var osImageDebianRegex = regexp.MustCompile(`^Debian GNU\/Linux ([0-9.]+)( )?(.*)?$`)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_os_version",
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: applyNodesMinimalOSVersionFilter,
		},
	},
}, discoverMinimalNodesOSVersion)

const (
	minVersionUbuntuValuesKey = "nodeManager:nodesMinimalOSVersionUbuntu"
	minVersionDebianValuesKey = "nodeManager:nodesMinimalOSVersionDebian"
)

func applyNodesMinimalOSVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "status", "nodeInfo", "osImage")
	return version, err
}

// Converts to semver format: 20.04.3 -> 20.4.3
func normalizeUbuntuVersionForSemver(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return version
	}
	major := strings.TrimLeft(parts[0], "0")
	if major == "" {
		major = "0"
	}
	minor := strings.TrimLeft(parts[1], "0")
	if minor == "" {
		minor = "0"
	}
	patch := strings.TrimLeft(parts[2], "0")
	if patch == "" {
		patch = "0"
	}
	return major + "." + minor + "." + patch
}

// Converts to Ubuntu format: minor is always two digits, patch without leading zeros
func normalizeUbuntuVersionForDisplay(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return version
	}
	major := parts[0]
	minor := parts[1]
	if len(minor) == 1 {
		minor = "0" + minor
	}
	patch := strings.TrimLeft(parts[2], "0")
	if patch == "" {
		patch = "0"
	}
	return major + "." + minor + "." + patch
}

func discoverMinimalNodesOSVersion(input *go_hook.HookInput) error {
	snaps := input.NewSnapshots.Get("nodes_os_version")
	if len(snaps) == 0 {
		return nil
	}

	var minUbuntuVersion, minDebianVersion *semver.Version
	for version, err := range sdkobjectpatch.SnapshotIter[string](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_os_version' snapshots: %w", err)
		}

		switch {
		case osImageUbuntuRegex.MatchString(version):
			ctrlUbuntuVersion, err := semver.NewVersion(osImageUbuntuRegex.FindStringSubmatch(version)[1])
			if err != nil {
				println("semver parse error for Ubuntu version:", semverStr, "error:", err.Error())
				return err
			}
			if minUbuntuVersion == nil || ctrlUbuntuVersion.LessThan(minUbuntuVersion) {
				minUbuntuVersion = ctrlUbuntuVersion
				minUbuntuVersionRaw = verStr
			}
		case osImageDebianRegex.MatchString(version):
			ctrlDebianVersion, err := semver.NewVersion(osImageDebianRegex.FindStringSubmatch(version)[1])
			if err != nil {
				return err
			}
			if minDebianVersion == nil || ctrlDebianVersion.LessThan(minDebianVersion) {
				minDebianVersion = ctrlDebianVersion
			}
		default:
			continue
		}
	}

	if minUbuntuVersion == nil {
		requirements.RemoveValue(minVersionUbuntuValuesKey)
	} else {
		// Save in Ubuntu format
		displayVersion := normalizeUbuntuVersionForDisplay(minUbuntuVersionRaw)
		requirements.SaveValue(minVersionUbuntuValuesKey, displayVersion)
	}
	if minDebianVersion == nil {
		requirements.RemoveValue(minVersionDebianValuesKey)
	} else {
		requirements.SaveValue(minVersionDebianValuesKey, minDebianVersion.String())
	}

	return nil
}
