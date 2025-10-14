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
	"context"
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

var osImageUbuntuRegex = regexp.MustCompile(`^Ubuntu ([0-9.]+)( )?(LTS)?$`)
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

// normalizeUbuntuVersionForSemver converts Ubuntu version format to semver format: 20.04.3 -> 20.4.3, 20.04 -> 20.4.0
func normalizeUbuntuVersionForSemver(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}

	// Normalize major version
	major := strings.TrimLeft(parts[0], "0")
	if major == "" {
		major = "0"
	}

	// Normalize minor version
	minor := strings.TrimLeft(parts[1], "0")
	if minor == "" {
		minor = "0"
	}

	// Handle patch version
	patch := "0"
	if len(parts) > 2 {
		patch = strings.TrimLeft(parts[2], "0")
		if patch == "" {
			patch = "0"
		}
	}

	return major + "." + minor + "." + patch
}

func discoverMinimalNodesOSVersion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("nodes_os_version")
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
			rawVersion := osImageUbuntuRegex.FindStringSubmatch(version)[1]
			normalizedVersion := normalizeUbuntuVersionForSemver(rawVersion)
			ctrlUbuntuVersion, err := semver.NewVersion(normalizedVersion)
			if err != nil {
				return err
			}
			if minUbuntuVersion == nil || ctrlUbuntuVersion.LessThan(minUbuntuVersion) {
				minUbuntuVersion = ctrlUbuntuVersion
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
		requirements.SaveValue(minVersionUbuntuValuesKey, minUbuntuVersion.String())
	}
	if minDebianVersion == nil {
		requirements.RemoveValue(minVersionDebianValuesKey)
	} else {
		requirements.SaveValue(minVersionDebianValuesKey, minDebianVersion.String())
	}

	return nil
}
