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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

const (
	nrtKeyPath            = "staticRoutingManager.internal.nodeRoutingTables"
	routingTableIDMinPath = "staticRoutingManager.routingTableIDMin"
	routingTableIDMaxPath = "staticRoutingManager.routingTableIDMax"
)

type RoutingTableInfo struct {
	Name             string
	UID              types.UID
	Generation       int64
	IsDeleted        bool
	IPRoutingTableID int
	Routes           []v1alpha1.Route
	NodeSelector     map[string]string
	Status           v1alpha1.RoutingTableStatus
}

type SDNInternalNodeRoutingTableInfo struct {
	Name      string
	IsDeleted bool
	NodeName  string
	Ready     bool
	Reason    string
}

type desiredNRTInfo struct {
	Name             string           `json:"name"`
	NodeName         string           `json:"nodeName"`
	OwnerRTName      string           `json:"ownerRTName"`
	OwnerRTUID       types.UID        `json:"ownerRTUID"`
	IPRoutingTableID int              `json:"ipRoutingTableID"`
	Routes           []v1alpha1.Route `json:"routes"`
}

type rtStatusPlus struct {
	v1alpha1.RoutingTableStatus
	failedNodes []string
	localErrors []string
}

type idIterator struct {
	UtilizedIDs map[int]struct{}
	IDSlider    int
	MaxID       int
}

func (i *idIterator) pickNextFreeID() (int, error) {
	if _, ok := i.UtilizedIDs[i.IDSlider]; ok {
		if i.IDSlider == i.MaxID {
			return 0, fmt.Errorf("ID pool for RoutingTable is over. Too many RoutingTables")
		}
		i.IDSlider++
		return i.pickNextFreeID()
	}
	i.UtilizedIDs[i.IDSlider] = struct{}{}
	return i.IDSlider, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "routingtables",
			ApiVersion: v1alpha1.GroupVersion,
			Kind:       v1alpha1.RTKind,
			FilterFunc: applyRoutingTablesFilter,
		},
		{
			Name:       "noderoutingtables",
			ApiVersion: v1alpha1.InternalGroupVersion,
			Kind:       v1alpha1.NRTKind,
			FilterFunc: applyNodeRoutingTablesFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: lib.ApplyNodeFilter,
		},
	},
}, routingTablesHandler)

func applyRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		rt     v1alpha1.RoutingTable
		result RoutingTableInfo
	)

	err := sdk.FromUnstructured(obj, &rt)
	if err != nil {
		return nil, err
	}

	result = RoutingTableInfo{
		Name:         rt.Name,
		UID:          rt.UID,
		Generation:   rt.Generation,
		IsDeleted:    rt.DeletionTimestamp != nil,
		Routes:       rt.Spec.Routes,
		NodeSelector: rt.Spec.NodeSelector,
		Status:       rt.Status,
	}

	switch rt.Spec.IPRoutingTableID != 0 {
	case true:
		result.IPRoutingTableID = rt.Spec.IPRoutingTableID
	case false:
		if rt.Status.IPRoutingTableID != 0 {
			result.IPRoutingTableID = rt.Status.IPRoutingTableID
		} else {
			result.IPRoutingTableID = 0
		}
	}

	return result, nil
}

func applyNodeRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		nrt    v1alpha1.SDNInternalNodeRoutingTable
		result SDNInternalNodeRoutingTableInfo
	)
	err := sdk.FromUnstructured(obj, &nrt)
	if err != nil {
		return nil, err
	}

	result.Name = nrt.Name
	result.IsDeleted = nrt.DeletionTimestamp != nil
	result.NodeName = nrt.Spec.NodeName

	cond := lib.FindStatusCondition(nrt.Status.Conditions, v1alpha1.ReconciliationSucceedType)
	if cond == nil {
		result.Ready = false
		result.Reason = v1alpha1.ReconciliationReasonPending
	} else {
		result.Ready = cond.Status == metav1.ConditionTrue
		result.Reason = cond.Reason
	}

	return result, nil
}

func routingTablesHandler(input *go_hook.HookInput) error {
	// Init vars
	routingTableIDMin, err := strconv.Atoi(input.Values.Get(routingTableIDMinPath).String())
	if err != nil {
		input.LogEntry.Errorf("unable to get routingTableIDMin from moduleConfig values")
		return nil
	}
	routingTableIDMax, err := strconv.Atoi(input.Values.Get(routingTableIDMaxPath).String())
	if err != nil {
		input.LogEntry.Errorf("unable to get routingTableIDMax from moduleConfig values")
		return nil
	}
	idi := idIterator{
		UtilizedIDs: make(map[int]struct{}),
		IDSlider:    routingTableIDMin,
		MaxID:       routingTableIDMax,
	}
	actualNodeRoutingTables := make(map[string]SDNInternalNodeRoutingTableInfo)
	allNodes := make(map[string]struct{})
	affectedNodes := make(map[string][]RoutingTableInfo)
	desiredRTStatus := make(map[string]rtStatusPlus)
	desiredNodeRoutingTables := make([]desiredNRTInfo, 0)

	// Filling allNodes
	for _, nodeRaw := range input.Snapshots["nodes"] {
		node := nodeRaw.(lib.NodeInfo)
		allNodes[node.Name] = struct{}{}
	}

	// Filling actualNodeRoutingTables and delete finalizers from orphan NRTs
	for _, nrtRaw := range input.Snapshots["noderoutingtables"] {
		nrtis := nrtRaw.(SDNInternalNodeRoutingTableInfo)
		actualNodeRoutingTables[nrtis.Name] = nrtis
		if _, ok := allNodes[nrtis.NodeName]; !ok && nrtis.IsDeleted {
			input.LogEntry.Infof("An orphan NRT %v was found. It will be deleted", nrtis.Name)
			lib.DeleteFinalizer(
				input,
				nrtis.Name,
				v1alpha1.InternalGroupVersion,
				v1alpha1.NRTKind,
				v1alpha1.Finalizer,
			)
		}
	}

	// Filling utilizedIDs
	for _, rtiRaw := range input.Snapshots["routingtables"] {
		rti := rtiRaw.(RoutingTableInfo)
		if rti.IPRoutingTableID != 0 {
			idi.UtilizedIDs[rti.IPRoutingTableID] = struct{}{}
		}
	}

	// main loop
	for _, rtiRaw := range input.Snapshots["routingtables"] {
		rti := rtiRaw.(RoutingTableInfo)

		// DRT stands for Desired Routing Table
		tmpDRTS := new(rtStatusPlus)
		tmpDRTS.failedNodes = make([]string, 0)
		tmpDRTS.localErrors = make([]string, 0)

		if _, ok := desiredRTStatus[rti.Name]; ok {
			*tmpDRTS = desiredRTStatus[rti.Name]
		}

		// Generate desired ObservedGeneration
		tmpDRTS.ObservedGeneration = rti.Generation

		// Generate desired IPRoutingTableID
		if rti.IPRoutingTableID == 0 {
			input.LogEntry.Infof("RoutingTable %v needs to be updated", rti.Name)
			tmpDRTS.IPRoutingTableID, err = idi.pickNextFreeID()
			if err != nil {
				input.LogEntry.Warnf("can't pick free ID for RoutingTable %v, error: %v", rti.Name, err)
				tmpDRTS.localErrors = append(tmpDRTS.localErrors, err.Error())
			}
		} else {
			tmpDRTS.IPRoutingTableID = rti.IPRoutingTableID
		}

		// Generate desired AffectedNodeRoutingTables and ReadyNodeRoutingTables, and filling affectedNodes
		validatedSelector, _ := labels.ValidatedSelectorFromSet(rti.NodeSelector)
		for _, nodeiRaw := range input.Snapshots["nodes"] {
			nodei := nodeiRaw.(lib.NodeInfo)

			if validatedSelector.Matches(labels.Set(nodei.Labels)) {
				tmpDRTS.AffectedNodeRoutingTables++
				nrtName := rti.Name + "-" + lib.GenerateShortHash(rti.Name+"#"+nodei.Name)
				if _, ok := actualNodeRoutingTables[nrtName]; ok {
					if actualNodeRoutingTables[nrtName].Ready {
						tmpDRTS.ReadyNodeRoutingTables++
					} else if actualNodeRoutingTables[nrtName].Reason == v1alpha1.ReconciliationReasonFailed {
						tmpDRTS.failedNodes = append(tmpDRTS.failedNodes, nodei.Name)
					}
				}

				rti.Status.IPRoutingTableID = tmpDRTS.IPRoutingTableID

				// Filling affectedNodes
				if rti.Status.IPRoutingTableID != 0 {
					// if 0, it means that the value has not been set yet, and the generation of a new one failed
					affectedNodes[nodei.Name] = append(affectedNodes[nodei.Name], rti)
				}
			}
		}

		// Generate desired conditions
		newCond := v1alpha1.ExtendedCondition{}
		t := metav1.NewTime(time.Now())
		if rti.Status.Conditions != nil {
			tmpDRTS.Conditions = rti.Status.Conditions
		} else {
			tmpDRTS.Conditions = make([]v1alpha1.ExtendedCondition, 0)
		}

		if len(tmpDRTS.localErrors) == 0 {
			if tmpDRTS.ReadyNodeRoutingTables == tmpDRTS.AffectedNodeRoutingTables {
				newCond.Type = v1alpha1.ReconciliationSucceedType
				newCond.LastHeartbeatTime = t
				newCond.Status = metav1.ConditionTrue
				newCond.Reason = v1alpha1.ReconciliationReasonSucceed
				newCond.Message = ""
			} else {
				if len(tmpDRTS.failedNodes) > 0 {
					newCond.Type = v1alpha1.ReconciliationSucceedType
					newCond.LastHeartbeatTime = t
					newCond.Status = metav1.ConditionFalse
					newCond.Reason = v1alpha1.ReconciliationReasonFailed
					newCond.Message = "Failed reconciling on " + strings.Join(tmpDRTS.failedNodes, ", ")
				} else {
					newCond.Type = v1alpha1.ReconciliationSucceedType
					newCond.LastHeartbeatTime = t
					newCond.Status = metav1.ConditionFalse
					newCond.Reason = v1alpha1.ReconciliationReasonPending
					newCond.Message = ""
				}
			}
		} else {
			newCond.Type = v1alpha1.ReconciliationSucceedType
			newCond.LastHeartbeatTime = t
			newCond.Status = metav1.ConditionFalse
			newCond.Reason = v1alpha1.ReconciliationReasonFailed
			newCond.Message = strings.Join(tmpDRTS.localErrors, "\n")
		}

		_ = lib.SetStatusCondition(&tmpDRTS.Conditions, newCond)

		desiredRTStatus[rti.Name] = *tmpDRTS
	}

	// Filling desiredNodeRoutingTables
	for nodeName, rtis := range affectedNodes {
		for _, rti := range rtis {
			var tmpNRTSpec desiredNRTInfo
			tmpNRTSpec.Name = rti.Name + "-" + lib.GenerateShortHash(rti.Name+"#"+nodeName)
			tmpNRTSpec.NodeName = nodeName
			tmpNRTSpec.OwnerRTName = rti.Name
			tmpNRTSpec.OwnerRTUID = rti.UID
			tmpNRTSpec.IPRoutingTableID = rti.Status.IPRoutingTableID
			tmpNRTSpec.Routes = rti.Routes
			desiredNodeRoutingTables = append(desiredNodeRoutingTables, tmpNRTSpec)
		}
	}
	// Sort desiredNodeRoutingTables to prevent helm flapping
	sort.SliceStable(desiredNodeRoutingTables, func(i, j int) bool {
		return desiredNodeRoutingTables[i].Name < desiredNodeRoutingTables[j].Name
	})
	input.Values.Set(nrtKeyPath, desiredNodeRoutingTables)

	// Update status in k8s
	for rtName, drts := range desiredRTStatus {
		newStatus := v1alpha1.RoutingTableStatus{}

		newStatus.ObservedGeneration = drts.ObservedGeneration
		newStatus.ReadyNodeRoutingTables = drts.ReadyNodeRoutingTables
		newStatus.AffectedNodeRoutingTables = drts.AffectedNodeRoutingTables
		if drts.IPRoutingTableID != 0 {
			newStatus.IPRoutingTableID = drts.IPRoutingTableID
		}
		newStatus.Conditions = drts.Conditions

		statusPatch := map[string]interface{}{
			"status": newStatus,
		}

		input.PatchCollector.MergePatch(
			statusPatch,
			v1alpha1.GroupVersion,
			v1alpha1.RTKind,
			"",
			rtName,
			object_patch.WithSubresource("/status"),
		)
	}

	return nil
}

// service functions
