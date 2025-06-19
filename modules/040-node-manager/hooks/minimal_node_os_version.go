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

func discoverMinimalNodesOSVersion(input *go_hook.HookInput) error {
	snaps := input.NewSnapshots.Get("nodes_os_version")
	if len(snaps) == 0 {
		return nil
	}

	var minUbuntuVersion, minDebianVersion *semver.Version
	for snap, err := range sdkobjectpatch.SnapshotIter[string](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_os_version' snapshots: %w", err)
		}

		switch {
		case osImageUbuntuRegex.MatchString(snap):
			ctrlUbuntuVersion, err := semver.NewVersion(osImageUbuntuRegex.FindStringSubmatch(snap)[1])
			if err != nil {
				return err
			}
			if minUbuntuVersion == nil || ctrlUbuntuVersion.LessThan(minUbuntuVersion) {
				minUbuntuVersion = ctrlUbuntuVersion
			}
		case osImageDebianRegex.MatchString(snap):
			ctrlDebianVersion, err := semver.NewVersion(osImageDebianRegex.FindStringSubmatch(snap)[1])
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
