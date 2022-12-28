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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

var osImageRegex = regexp.MustCompile(`^Ubuntu ([0-9.]+)( )?(LTS)?$`)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_os_version",
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(true),
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
}, discoverMinimalNodesOSVersiopon)

const (
	minVersionValuesKey = "nodeManager:nodesMinimalOSVersionUbuntu"
)

func applyNodesMinimalOSVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "status", "nodeInfo", "osImage")
	return version, err
}

func discoverMinimalNodesOSVersiopon(input *go_hook.HookInput) error {
	snap := input.Snapshots["nodes_os_version"]
	if len(snap) == 0 {
		return nil
	}

	var minVersion *semver.Version

	for _, s := range snap {
		if !osImageRegex.MatchString(s.(string)) {
			continue
		}
		ctrlVersion, err := semver.NewVersion(osImageRegex.FindStringSubmatch(s.(string))[1])
		if err != nil {
			return err
		}
		if minVersion == nil || ctrlVersion.LessThan(minVersion) {
			minVersion = ctrlVersion
		}
	}

	if minVersion == nil {
		requirements.RemoveValue(minVersionValuesKey)
		return nil
	}

	requirements.SaveValue(minVersionValuesKey, minVersion.String())
	return nil
}
