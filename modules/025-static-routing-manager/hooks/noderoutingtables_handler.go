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
	"reflect"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

const (
	nodeNameLabel = "routing-manager.network.deckhouse.io/node-name"
	ownerRTLabel  = "routing-manager.network.deckhouse.io/owner-routing-table-claim-name"
	finalizer     = "routing-tables-manager.network.deckhouse.io"
)

type nrtsPlus struct {
	rtName string
	rtUID  types.UID
	spec   v1alpha1.NodeRoutingTableSpec
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "routetables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "RoutingTable",
			FilterFunc: applyMainHandlerRouteTablesFilter,
		},
		{
			Name:       "noderoutetables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "NodeRoutingTable",
			FilterFunc: applyMainHandlerNodeRouteTablesFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyMainHandlerNodeFilter,
		},
	},
}, nodeRoutingTablesHandler)

func applyMainHandlerRouteTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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
	result.IPRouteTableID = rt.Status.IPRouteTableID
	result.Routes = rt.Spec.Routes
	result.NodeSelector = rt.Spec.NodeSelector

	return result, nil
}

func applyMainHandlerNodeRouteTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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
	// prepare
	var (
		nodeRoutingTablesToCreate []*v1alpha1.NodeRoutingTable
		nodeRoutingTablesToUpdate []*v1alpha1.NodeRoutingTable
		nodeRoutingTablesToDelete []string
	)

	// main logic start

	// Filling affectedNodes
	affectedNodes := make(map[string][]lib.RoutingTableInfo)
	for _, rtiRaw := range input.Snapshots["routetables"] {
		rti := rtiRaw.(lib.RoutingTableInfo)
		if rti.IPRouteTableID == 0 {
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
	for _, nrtRaw := range input.Snapshots["noderoutetables"] {
		nrtis := nrtRaw.(lib.NodeRoutingTableInfo)
		actualNodeRoutingTables[nrtis.Name] = nrtis.NodeRoutingTable
	}

	// Filling desiredNodeRoutingTables
	desiredNodeRoutingTables := make(map[string]nrtsPlus)
	for nodeName, rtis := range affectedNodes {
		for _, rti := range rtis {
			var tmpNRTS nrtsPlus
			tmpNRTS.rtName = rti.Name
			tmpNRTS.rtUID = rti.UID
			tmpNRTS.spec.NodeName = nodeName
			tmpNRTS.spec.IPRouteTableID = rti.IPRouteTableID
			tmpNRTS.spec.Routes = rti.Routes
			tmpNRTName := rti.Name + "-" + generateShortHash(rti.Name+"#"+nodeName)
			desiredNodeRoutingTables[tmpNRTName] = tmpNRTS
		}
	}

	// Filling actions tasks
	for nrtName, ntrps := range desiredNodeRoutingTables {
		if _, ok := actualNodeRoutingTables[nrtName]; ok {
			if reflect.DeepEqual(ntrps.spec, actualNodeRoutingTables[nrtName]) {
				continue
			}
			nrt := generateNRT(nrtName, ntrps)
			nodeRoutingTablesToUpdate = append(nodeRoutingTablesToUpdate, nrt)
		} else {
			nrt := generateNRT(nrtName, ntrps)
			nodeRoutingTablesToCreate = append(nodeRoutingTablesToCreate, nrt)
		}
	}
	for nrtName := range actualNodeRoutingTables {
		if _, ok := desiredNodeRoutingTables[nrtName]; !ok {
			nodeRoutingTablesToDelete = append(nodeRoutingTablesToDelete, nrtName)
		}
	}

	for _, nrt := range nodeRoutingTablesToUpdate {
		input.PatchCollector.Create(nrt, object_patch.UpdateIfExists())
	}
	for _, nrt := range nodeRoutingTablesToCreate {
		input.PatchCollector.Create(nrt, object_patch.IgnoreIfExists())
	}
	for _, nrtName := range nodeRoutingTablesToDelete {
		input.PatchCollector.Delete(lib.GroupVersion, lib.NRTKind, "", nrtName, object_patch.InForeground())
	}

	// main logic end

	return nil
}

// service functions

func generateNRT(name string, nrtps nrtsPlus) *v1alpha1.NodeRoutingTable {
	nrt := &v1alpha1.NodeRoutingTable{
		TypeMeta: metav1.TypeMeta{
			Kind:       lib.NRTKind,
			APIVersion: lib.GroupVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodeNameLabel: nrtps.spec.NodeName,
				ownerRTLabel:  nrtps.rtName,
			},
			Finalizers: []string{
				finalizer,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         lib.NRTKind,
					Kind:               lib.RTKind,
					Name:               nrtps.rtName,
					UID:                nrtps.rtUID,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: nrtps.spec,
	}

	return nrt
}

func generateShortHash(input string) string {
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	if len(fullHash) > 10 {
		return fullHash[:10]
	}
	return fullHash
}
