/*
Copyright 2024 Flant JSC

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
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

const (
	nodeNameLabel = "routing-manager.network.deckhouse.io/node-name"
	finalizer     = "routing-tables-manager.network.deckhouse.io"
	nrtKeyPath    = "staticRoutingManager.internal.nodeRoutingTables"
)

type nrtsPlus struct {
	rtName string
	rtUID  types.UID
	spec   v1alpha1.NodeRoutingTableSpec
}

type desiredNRTInfo struct {
	Name             string           `json:"name"`
	NodeName         string           `json:"nodeName"`
	OwnerRTName      string           `json:"ownerRTName"`
	OwnerRTUID       types.UID        `json:"ownerRTUID"`
	IPRoutingTableID int              `json:"ipRoutingTableID"`
	Routes           []v1alpha1.Route `json:"routes"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "routingtables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "RoutingTable",
			FilterFunc: applyMainHandlerRoutingTablesFilter,
		},
		{
			Name:       "noderoutingtables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "NodeRoutingTable",
			FilterFunc: applyMainHandlerNodeRoutingTablesFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyMainHandlerNodeFilter,
		},
	},
}, nodeRoutingTablesHandler)

func applyMainHandlerRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		rt     v1alpha1.RoutingTable
		result lib.RoutingTableInfo
	)
	err := sdk.FromUnstructured(obj, &rt)
	if err != nil {
		return nil, err
	}

	result.Name = rt.Name
	result.UID = rt.UID
	result.IPRoutingTableID = rt.Status.IPRoutingTableID
	result.Routes = rt.Spec.Routes
	result.NodeSelector = rt.Spec.NodeSelector

	return result, nil
}

func applyMainHandlerNodeRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		nrt    v1alpha1.NodeRoutingTable
		result lib.NodeRoutingTableInfo
	)
	err := sdk.FromUnstructured(obj, &nrt)
	if err != nil {
		return nil, err
	}

	result.Name = nrt.Name
	result.NodeRoutingTable = nrt.Spec

	return result, nil
}

func applyMainHandlerNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		node   v1.Node
		result lib.NodeInfo
	)
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result.Name = node.Name
	result.Labels = node.Labels

	return result, nil
}

func nodeRoutingTablesHandler(input *go_hook.HookInput) error {

	// main logic start

	// Filling affectedNodes
	affectedNodes := make(map[string][]lib.RoutingTableInfo)
	for _, rtiRaw := range input.Snapshots["routingtables"] {
		rti := rtiRaw.(lib.RoutingTableInfo)
		if rti.IPRoutingTableID == 0 {
			// status.ipRouteTableID isn't set yet
			continue
		}
		validatedSelector, _ := labels.ValidatedSelectorFromSet(rti.NodeSelector)
		for _, nodeiRaw := range input.Snapshots["nodes"] {
			nodei := nodeiRaw.(lib.NodeInfo)
			if validatedSelector.Matches(labels.Set(nodei.Labels)) {
				affectedNodes[nodei.Name] = append(affectedNodes[nodei.Name], rti)
			}
		}
	}

	// Filling actualNodeRoutingTables
	actualNodeRoutingTables := make(map[string]v1alpha1.NodeRoutingTableSpec)
	for _, nrtRaw := range input.Snapshots["noderoutingtables"] {
		nrtis := nrtRaw.(lib.NodeRoutingTableInfo)
		actualNodeRoutingTables[nrtis.Name] = nrtis.NodeRoutingTable
	}

	// Filling desiredNodeRoutingTables
	var desiredNodeRoutingTables []desiredNRTInfo
	for nodeName, rtis := range affectedNodes {
		for _, rti := range rtis {
			var tmpNRTS desiredNRTInfo
			tmpNRTS.Name = rti.Name + "-" + generateShortHash(rti.Name+"#"+nodeName)
			tmpNRTS.NodeName = nodeName
			tmpNRTS.OwnerRTName = rti.Name
			tmpNRTS.OwnerRTUID = rti.UID
			tmpNRTS.IPRoutingTableID = rti.IPRoutingTableID
			tmpNRTS.Routes = rti.Routes
			desiredNodeRoutingTables = append(desiredNodeRoutingTables, tmpNRTS)
		}
	}

	sort.SliceStable(desiredNodeRoutingTables, func(i, j int) bool {
		return desiredNodeRoutingTables[i].Name < desiredNodeRoutingTables[j].Name
	})

	if len(desiredNodeRoutingTables) > 0 {
		input.Values.Set(nrtKeyPath, desiredNodeRoutingTables)
	}

	// main logic end

	return nil
}

// service functions

func generateShortHash(input string) string {
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	if len(fullHash) > 10 {
		return fullHash[:10]
	}
	return fullHash
}
