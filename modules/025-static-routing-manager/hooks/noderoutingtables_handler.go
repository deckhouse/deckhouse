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
	"reflect"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

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
			Kind:       "NodeRoutingTables",
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
	result.IPRouteTableID = rt.Status.IPRouteTableID
	result.Routes = rt.Spec.Routes
	result.NodeSelector = rt.Spec.NodeSelector

	return result, nil
}

func applyMainHandlerNodeRouteTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		nrt    v1alpha1.NodeRoutingTables
		result lib.NodeRoutingTableInfo
	)
	err := sdk.FromUnstructured(obj, &nrt)
	if err != nil {
		return nil, err
	}

	result.Name = nrt.Name
	result.NodeRoutingTables = nrt.Spec

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
		// affectedNodes             map[string][]RoutingTableInfo
		// nodeRoutingTablesCache map[string]v1alpha1.NodeRoutingTablesSpec
		// desiredNodeRoutingTables map[string]v1alpha1.NodeRoutingTablesSpec
		// actualNodeRoutingTables   map[string]v1alpha1.NodeRoutingTablesSpec
		nodeRoutingTablesToCreate []*v1alpha1.NodeRoutingTables
		nodeRoutingTablesToUpdate []*v1alpha1.NodeRoutingTables
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
	actualNodeRoutingTables := make(map[string]v1alpha1.NodeRoutingTablesSpec)
	for _, nrtRaw := range input.Snapshots["noderoutetables"] {
		nrtis := nrtRaw.(lib.NodeRoutingTableInfo)
		actualNodeRoutingTables[nrtis.Name] = nrtis.NodeRoutingTables
	}

	// Filling desiredNodeRoutingTables
	desiredNodeRoutingTables := make(map[string]v1alpha1.NodeRoutingTablesSpec)
	nodeRoutingTablesCache := make(map[string]v1alpha1.NodeRoutingTablesSpec)
	for nodeName, rtis := range affectedNodes {
		hash := getHash(rtis)

		if _, ok := nodeRoutingTablesCache[hash]; ok {
			desiredNodeRoutingTables[nodeName] = nodeRoutingTablesCache[hash]
		} else {
			tmpNRTS := new(v1alpha1.NodeRoutingTablesSpec)
			// var nrts v1alpha1.NodeRoutingTablesSpec
			for _, rti := range rtis {
				lib.NRTSAppend(tmpNRTS, rti)
			}
			nodeRoutingTablesCache[hash] = *tmpNRTS
			desiredNodeRoutingTables[nodeName] = *tmpNRTS
		}
	}

	// Filling actions tasks
	for nodeName, ntrs := range desiredNodeRoutingTables {
		if _, ok := actualNodeRoutingTables[nodeName]; ok {
			if reflect.DeepEqual(ntrs, nodeRoutingTablesCache[nodeName]) {
				continue
			}
			nrt := generateNRT(nodeName, ntrs)
			nodeRoutingTablesToUpdate = append(nodeRoutingTablesToUpdate, nrt)
		} else {
			nrt := generateNRT(nodeName, ntrs)
			nodeRoutingTablesToCreate = append(nodeRoutingTablesToCreate, nrt)
		}
	}
	for nodeName, ntrs := range actualNodeRoutingTables {
		if _, ok := desiredNodeRoutingTables[nodeName]; ok {
			continue
		}
		nrt := generateNRT(nodeName, ntrs)
		nodeRoutingTablesToDelete = append(nodeRoutingTablesToDelete, nrt.Name)
	}

	for _, nrt := range nodeRoutingTablesToUpdate {
		// unnrt, _ := json.Marshal(nrt)
		input.PatchCollector.Create(nrt, object_patch.UpdateIfExists())
	}
	for _, nrt := range nodeRoutingTablesToCreate {
		// unnrt, _ := json.Marshal(nrt)
		input.PatchCollector.Create(nrt, object_patch.IgnoreIfExists())
	}
	for _, nrtName := range nodeRoutingTablesToDelete {
		input.PatchCollector.Delete(lib.GroupVersion, lib.NRTKind, "", nrtName, object_patch.InForeground())
	}

	// main logic end

	return nil
}

// service functions

func getHash(rtis []lib.RoutingTableInfo) string {
	var tmp []string
	for _, rt := range rtis {
		tmp = append(tmp, rt.Name)
	}
	sort.Strings(tmp)
	return strings.Join(tmp, ":")
}

func generateNRT(name string, nrts v1alpha1.NodeRoutingTablesSpec) *v1alpha1.NodeRoutingTables {
	nrt := &v1alpha1.NodeRoutingTables{
		TypeMeta: metav1.TypeMeta{
			Kind:       lib.NRTKind,
			APIVersion: lib.GroupVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: nrts,
	}

	return nrt
}
