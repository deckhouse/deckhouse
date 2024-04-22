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
	"math/rand"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

const (
	RouteTableIDMin int = 10000
	RouteTableIDMax int = 11000
)

type routingTableInfo struct {
	Name                    string
	DeletionTimestampExists bool
	SpecIPRouteTableID      int
	StatusIPRouteTableID    int
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "routetables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "RoutingTable",
			FilterFunc: applyRouteTablesFilter,
		},
	},
}, routingTableStatusIDHandler)

func applyRouteTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		rt     v1alpha1.RoutingTable
		result routingTableInfo
	)
	err := sdk.FromUnstructured(obj, &rt)
	if err != nil {
		return nil, err
	}

	result.Name = rt.Name
	result.DeletionTimestampExists = rt.DeletionTimestamp != nil

	if rt.Spec.IPRouteTableID == 0 {
		result.SpecIPRouteTableID = 0
	} else {
		result.SpecIPRouteTableID = rt.Spec.IPRouteTableID
	}

	if rt.Status.IPRouteTableID == 0 {
		result.StatusIPRouteTableID = 0
	} else {
		result.StatusIPRouteTableID = rt.Status.IPRouteTableID
	}

	return result, nil
}

func routingTableStatusIDHandler(input *go_hook.HookInput) error {
	var newRTId int

	busyIDs := make(map[int]struct{})
	for _, rtiRaw := range input.Snapshots["routetables"] {
		rti := rtiRaw.(routingTableInfo)
		busyIDs[rti.StatusIPRouteTableID] = struct{}{}
	}

	for _, rtiRaw := range input.Snapshots["routetables"] {
		rti := rtiRaw.(routingTableInfo)

		if !shouldUpdateStatusRouteTableID(rti, input.LogEntry) {
			continue
		}
		input.LogEntry.Infof("RoutingTable %v needs to be updated", rti.Name)

		if rti.SpecIPRouteTableID != 0 {
			newRTId = rti.SpecIPRouteTableID
		} else {
			newRTId = generateFreeRoutingTableID(busyIDs)

			busyIDs[newRTId] = struct{}{}
		}

		statusPatch := map[string]interface{}{
			"status": v1alpha1.RoutingTableStatus{
				IPRouteTableID: newRTId,
			},
		}
		input.PatchCollector.MergePatch(
			statusPatch,
			lib.GroupVersion,
			lib.RTKind,
			"",
			rti.Name,
			object_patch.WithSubresource("/status"),
		)
	}
	return nil
}

// service functions

func shouldUpdateStatusRouteTableID(rti routingTableInfo, log *logrus.Entry) bool {
	if rti.DeletionTimestampExists {
		return false
	}

	if rti.StatusIPRouteTableID == 0 {
		log.Infof("In RoutingTable %v status.IPRouteTableID is empty", rti.Name)
		return true
	}

	if rti.StatusIPRouteTableID != 0 && rti.SpecIPRouteTableID == 0 {
		return false
	}

	if rti.SpecIPRouteTableID != 0 && rti.SpecIPRouteTableID == rti.StatusIPRouteTableID {
		return false
	}

	log.Infof("RoutingTable %v is not in the deletion status, status.IPRouteTableID and spec.IPRouteTableID are not empty, but not are equal", rti.Name)
	return true
}

func generateFreeRoutingTableID(busyIDs map[int]struct{}) int {
	for {
		randomizer := rand.New(rand.NewSource(time.Now().UnixNano()))
		newRTId := randomizer.Intn(RouteTableIDMax-RouteTableIDMin) + RouteTableIDMin
		if _, ok := busyIDs[newRTId]; ok {
			continue
		}
		return newRTId
	}
}
