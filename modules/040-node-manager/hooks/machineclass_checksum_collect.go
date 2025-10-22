/*
Copyright 2021 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

/*

DESIGN
	- BEFORE HELM {subscribed to MachineDeployments} (you are here)
		collects checksums of all EXISTING MachineClasses to the map
			nodeManager.internal.machineDeployments:
				"{MachineDeployment name}": "{name, nodeGroup, Checksum}"
		- MachineDeployments in snapshot are always expected to have MachineClass checksum in annotations
		- nodeManager.internal.nodeGroups are always expected to exist

	- HELM in MachineDeployment template, the checksum is set from values
		If the checksum is absent in values, it means the MachineDeployment is being created,
		and the checksum is calculated right in the template.

	- AFTER HELM {}
		updates checksums
		- sets checksums in MachineDeployments specs causing nodes to update, if it changes
		- updates checksums in the values

*/

const (
	machineDeploymentsInternalValuesPath = "nodeManager.internal.machineDeployments"
)

// nodeManager.internal.nodeGroups is expected to be fulfilled by the time of the call,
// hence `get_crds` hook should execute before this one.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "machine_deployments",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineDeployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterMachineDeploymentForChecksumCalculation,
		},
	},
}, saveMachineClassChecksum)

type machineDeployment struct {
	// Name of the MachineDeployment, for convenience
	Name string `json:"name"`
	// NodeGroup tracks the relation to between a NodeGroup
	NodeGroup string `json:"nodeGroup"`
	// Checksum of the MachineClass, to be reused in MachineDeployment templates at right moments
	Checksum string `json:"checksum"`
}

func filterMachineDeploymentForChecksumCalculation(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()

	nodeGroup, err := pickStringField(obj, "metadata", "labels", "node-group")
	if err != nil {
		return nil, fmt.Errorf(`unexpected state of MachineDeployment %q: %v`, name, err)
	}

	checksum, err := pickStringField(obj, "spec", "template", "metadata", "annotations", "checksum/machine-class")
	if err != nil {
		return nil, fmt.Errorf(`unexpected state of MachineDeployment %q: %v`, name, err)
	}

	result := machineDeployment{
		Name:      name,
		NodeGroup: nodeGroup,
		Checksum:  checksum,
	}

	return result, nil
}

func saveMachineClassChecksum(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Exists(machineDeploymentsInternalValuesPath) {
		input.Values.Set(machineDeploymentsInternalValuesPath, map[string]interface{}{})
	}

	rawMDs := input.Snapshots.Get("machine_deployments")
	if len(rawMDs) == 0 {
		return nil
	}

	ngs, err := parseNodeGroupValues(input.Values)
	if err != nil {
		return fmt.Errorf("cannot parse nodeGroup values: %v", err)
	}
	for md, err := range sdkobjectpatch.SnapshotIter[machineDeployment](rawMDs) {
		if err != nil {
			return fmt.Errorf("cannot parse machineDeployment filter result")
		}

		key := fmt.Sprintf("%s.%s", machineDeploymentsInternalValuesPath, md.Name)

		ng := chooseNodeGroupByMachineDeployment(ngs, md)
		if ng == nil {
			// No NodeGroup value for MachineDeployment means we should clean up.
			input.Values.Remove(key)
			continue
		}

		input.Values.Set(key, md)
	}

	return nil
}

type nodeGroupValue struct {
	Name string        `json:"name"`
	Type ngv1.NodeType `json:"nodeType"`
	Raw  interface{}   `json:"-"`
}

func parseNodeGroupValues(values sdkpkg.PatchableValuesCollector) ([]*nodeGroupValue, error) {
	const nodeGroupsPath = "nodeManager.internal.nodeGroups"
	var ng []*nodeGroupValue

	ngsJSON := values.Get(nodeGroupsPath)
	if !ngsJSON.Exists() {
		return ng, nil
	}

	err := json.Unmarshal([]byte(ngsJSON.Raw), &ng)
	if err != nil {
		return nil, err
	}
	for i := range ng {
		indexKey := fmt.Sprintf("%s.%d", nodeGroupsPath, i)
		ng[i].Raw = values.Get(indexKey).Value()
	}
	return ng, nil
}

func chooseNodeGroupByMachineDeployment(ngs []*nodeGroupValue, md machineDeployment) *nodeGroupValue {
	for _, ng := range ngs {
		if ng.Type != ngv1.NodeTypeCloudEphemeral {
			continue
		}

		if ng.Name != md.NodeGroup {
			continue
		}

		return ng
	}
	return nil
}

func pickStringField(obj *unstructured.Unstructured, fields ...string) (string, error) {
	value, ok, err := unstructured.NestedString(obj.Object, fields...)
	path := strings.Join(fields, ".")
	if !ok {
		return "", fmt.Errorf(`expected field %s to be set`, path)
	}
	if err != nil {
		return "", fmt.Errorf(`value of field %s is not a string: %v`, path, err)
	}
	return value, nil
}
