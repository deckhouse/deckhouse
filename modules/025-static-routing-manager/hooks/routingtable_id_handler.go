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
	RoutingTableIDMin int = 10000
	RoutingTableIDMax int = 11000
)

type routingTableInfo struct {
	Name                   string
	IsDeleted              bool
	SpecIPRoutingTableID   int
	StatusIPRoutingTableID int
}

type idIterator struct {
	UtilizedIDs       map[int]struct{}
	LastFreeIDByOrder int
}

func (i *idIterator) pickNextFreeID() (int, error) {
	if _, ok := i.UtilizedIDs[i.LastFreeIDByOrder]; ok {
		i.LastFreeIDByOrder++
		return i.pickNextFreeID()
	}
	i.UtilizedIDs[i.LastFreeIDByOrder] = struct{}{}
	return i.LastFreeIDByOrder, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "routingtables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "RoutingTable",
			FilterFunc: applyRoutingTablesFilter,
		},
	},
}, routingTableStatusIDHandler)

func applyRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		rt     v1alpha1.RoutingTable
		result routingTableInfo
	)
	err := sdk.FromUnstructured(obj, &rt)
	if err != nil {
		return nil, err
	}

	result.Name = rt.Name
	result.IsDeleted = rt.DeletionTimestamp != nil
	result.SpecIPRoutingTableID = rt.Spec.IPRoutingTableID
	result.StatusIPRoutingTableID = rt.Status.IPRoutingTableID

	return result, nil
}

func routingTableStatusIDHandler(input *go_hook.HookInput) error {
	var newRTId int

	utilizedIDs := make(map[int]struct{})
	for _, rtiRaw := range input.Snapshots["routingtables"] {
		rti := rtiRaw.(routingTableInfo)
		utilizedIDs[rti.StatusIPRoutingTableID] = struct{}{}
	}

	for _, rtiRaw := range input.Snapshots["routingtables"] {
		rti := rtiRaw.(routingTableInfo)

		if !shouldUpdateStatusRoutingTableID(rti, input.LogEntry) {
			continue
		}
		input.LogEntry.Infof("RoutingTable %v needs to be updated", rti.Name)

		if rti.SpecIPRoutingTableID != 0 {
			newRTId = rti.SpecIPRoutingTableID
		} else {
			newRTId = pickFreeRoutingTableID(utilizedIDs)

			utilizedIDs[newRTId] = struct{}{}
		}

		statusPatch := map[string]interface{}{
			"status": v1alpha1.RoutingTableStatus{
				IPRoutingTableID: newRTId,
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

func shouldUpdateStatusRoutingTableID(rti routingTableInfo, log *logrus.Entry) bool {
	if rti.IsDeleted {
		return false
	}

	if rti.StatusIPRoutingTableID == 0 {
		log.Infof("In RoutingTable %v status.IPRoutingTableID is empty", rti.Name)
		return true
	}

	if rti.StatusIPRoutingTableID != 0 && rti.SpecIPRoutingTableID == 0 {
		return false
	}

	if rti.SpecIPRoutingTableID != 0 && rti.SpecIPRoutingTableID == rti.StatusIPRoutingTableID {
		return false
	}

	log.Infof("RoutingTable %v is not in the deletion status, status.IPRoutingTableID and spec.IPRoutingTableID are not empty, but not are equal", rti.Name)
	return true
}

func pickFreeRoutingTableID(utilizedIDs map[int]struct{}) int {
	for {
		randomizer := rand.New(rand.NewSource(time.Now().UnixNano()))
		newRTId := randomizer.Intn(RoutingTableIDMax-RoutingTableIDMin) + RoutingTableIDMin
		if _, ok := utilizedIDs[newRTId]; ok {
			continue
		}
		return newRTId
	}
}
