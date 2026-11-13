/*
Copyright 2026 Flant JSC

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

package fencing

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	moduleQueue = "/modules/node-manager"

	modeDisabled = "Disabled"
	modeNormal   = "Normal"
	modeSlow     = "Slow"
	modeCritical = "Critical"

	nodeGroupsSnapshot = "node_groups"
)

// nodeGroupFencing is the part of a NodeGroup that the fencing hooks read.
type nodeGroupFencing struct {
	Name            string
	Mode            string
	WatchdogTimeout string
	Engine          bool
}

func applyNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mode, _, err := unstructured.NestedString(obj.Object, "spec", "fencing", "mode")
	if err != nil {
		return nil, err
	}
	timeout, _, err := unstructured.NestedString(obj.Object, "spec", "fencing", "watchdog", "timeout")
	if err != nil {
		return nil, err
	}

	systemType, _, err := unstructured.NestedString(obj.Object, "spec", "systemType")
	if err != nil {
		return nil, err
	}

	return nodeGroupFencing{
		Name:            obj.GetName(),
		Mode:            mode,
		WatchdogTimeout: timeout,
		Engine:          systemType == "Immutable",
	}, nil
}

var nodeGroupsBinding = go_hook.KubernetesConfig{
	Name:       nodeGroupsSnapshot,
	ApiVersion: "deckhouse.io/v1",
	Kind:       "NodeGroup",
	FilterFunc: applyNodeGroupFilter,
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes:   []go_hook.KubernetesConfig{nodeGroupsBinding},
}, handleNodeGroupModes)

// normalizeMode maps the mode of a NodeGroup to the mode that all consumers use.
// Watchdog is the deprecated name of Slow. Critical is available only in the
// commercial editions. In CE, a group with Critical works as Normal. Disabled
// covers the old values off and Notify and any value this release does not know.
func normalizeMode(mode, edition string) string {
	switch mode {
	case modeCritical:
		if edition == "CE" {
			return modeNormal
		}
		return mode
	case modeNormal, modeSlow:
		return mode
	case "Watchdog":
		return modeSlow
	default:
		return modeDisabled
	}
}

// handleNodeGroupModes writes the mode and the Engine flag (systemType Immutable) of every NodeGroup. It is the only writer of nodeManager.internal.fencing.nodeGroups.
func handleNodeGroupModes(_ context.Context, input *go_hook.HookInput) error {
	edition := input.Values.Get("global.deckhouseEdition").String()
	nodeGroups := make(map[string]map[string]any)
	for ng, err := range sdkobjectpatch.SnapshotIter[nodeGroupFencing](input.Snapshots.Get(nodeGroupsSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", nodeGroupsSnapshot, err)
		}
		nodeGroups[ng.Name] = map[string]any{"mode": normalizeMode(ng.Mode, edition), "engine": ng.Engine}
	}

	input.Values.Set("nodeManager.internal.fencing.nodeGroups", nodeGroups)
	return nil
}
