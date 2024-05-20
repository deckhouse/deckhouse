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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

const (
	Group                 = "network.deckhouse.io"
	Version               = "v1alpha1"
	GroupVersion          = Group + "/" + Version
	RTKind                = "RoutingTable"
	NRTKind               = "NodeRoutingTable"
	finalizer             = "routing-tables-manager.network.deckhouse.io"
	nrtKeyPath            = "staticRoutingManager.internal.nodeRoutingTables"
	RoutingTableIDMin int = 10000
	RoutingTableIDMax int = 11000
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

type NodeRoutingTableInfo struct {
	Name      string
	IsDeleted bool
	NodeName  string
	Ready     bool
	Reason    string
}

type NodeInfo struct {
	Name   string
	Labels map[string]string
}

type desiredNRTInfo struct {
	Name             string           `json:"name"`
	NodeName         string           `json:"nodeName"`
	OwnerRTName      string           `json:"ownerRTName"`
	OwnerRTUID       types.UID        `json:"ownerRTUID"`
	IPRoutingTableID int              `json:"ipRoutingTableID"`
	Routes           []v1alpha1.Route `json:"routes"`
}

type rtsPlus struct {
	v1alpha1.RoutingTableStatus
	failedNodes []string
	localErrors []string
}

type idIterator struct {
	UtilizedIDs map[int]struct{}
	IDSlider    int
}

func (i *idIterator) pickNextFreeID() (int, error) {
	if _, ok := i.UtilizedIDs[i.IDSlider]; ok {
		if i.IDSlider == RoutingTableIDMax {
			return 0, fmt.Errorf("ID pool for RoutingTable is over. Too many RoutingTables")
		}
		i.IDSlider++
		return i.pickNextFreeID()
	}
	i.UtilizedIDs[i.IDSlider] = struct{}{}
	return i.IDSlider, nil
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
		{
			Name:       "noderoutingtables",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "NodeRoutingTable",
			FilterFunc: applyNodeRoutingTablesFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
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
	if rt.Spec.IPRoutingTableID != 0 {
		result.IPRoutingTableID = rt.Spec.IPRoutingTableID
	} else if rt.Status.IPRoutingTableID != 0 {
		result.IPRoutingTableID = rt.Status.IPRoutingTableID
	} else {
		result.IPRoutingTableID = 0
	}

	return result, nil
}

func applyNodeRoutingTablesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		nrt    v1alpha1.NodeRoutingTable
		result NodeRoutingTableInfo
	)
	err := sdk.FromUnstructured(obj, &nrt)
	if err != nil {
		return nil, err
	}

	result.Name = nrt.Name
	result.IsDeleted = nrt.DeletionTimestamp != nil
	result.NodeName = nrt.Spec.NodeName

	cond := NRTFindStatusCondition(nrt.Status.Conditions, v1alpha1.ReconciliationSucceedType)
	if cond == nil {
		result.Ready = false
		result.Reason = v1alpha1.ReconciliationReasonPending
	} else {
		result.Ready = cond.Status == metav1.ConditionTrue
		result.Reason = cond.Reason
	}

	return result, nil
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		node   v1.Node
		result NodeInfo
	)
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result.Name = node.Name
	result.Labels = node.Labels

	return result, nil
}

func routingTablesHandler(input *go_hook.HookInput) error {
	var err error
	actualNodeRoutingTables := make(map[string]NodeRoutingTableInfo)
	allNodes := make(map[string]struct{})
	affectedNodes := make(map[string][]RoutingTableInfo)
	desiredRTStatus := make(map[string]rtsPlus)
	desiredNodeRoutingTables := make([]desiredNRTInfo, 0)
	idi := idIterator{
		UtilizedIDs: make(map[int]struct{}),
		IDSlider:    RoutingTableIDMin,
	}

	// Filling allNodes
	for _, nodeRaw := range input.Snapshots["nodes"] {
		node := nodeRaw.(NodeInfo)
		allNodes[node.Name] = struct{}{}
	}

	// Filling actualNodeRoutingTables and delete finalizers from orphan NRTs
	for _, nrtRaw := range input.Snapshots["noderoutingtables"] {
		nrtis := nrtRaw.(NodeRoutingTableInfo)
		actualNodeRoutingTables[nrtis.Name] = nrtis
		if _, ok := allNodes[nrtis.NodeName]; !ok && nrtis.IsDeleted {
			deleteFinalizerFromNRT(input, nrtis.Name)
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

		// if should to do then set pending status, else continue

		tmpDRTS := new(rtsPlus)
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
			nodei := nodeiRaw.(NodeInfo)

			if validatedSelector.Matches(labels.Set(nodei.Labels)) {
				tmpDRTS.AffectedNodeRoutingTables++
				nrtName := rti.Name + "-" + generateShortHash(rti.Name+"#"+nodei.Name)
				if _, ok := actualNodeRoutingTables[nrtName]; ok {
					if actualNodeRoutingTables[nrtName].Ready {
						tmpDRTS.ReadyNodeRoutingTables++
					} else if actualNodeRoutingTables[nrtName].Reason == v1alpha1.ReconciliationReasonFailed {
						tmpDRTS.failedNodes = append(tmpDRTS.failedNodes, nodei.Name)
					}
				}

				// Filling affectedNodes
				if rti.Status.IPRoutingTableID != 0 {
					// if 0 - status.ipRouteTableID isn't set yet
					affectedNodes[nodei.Name] = append(affectedNodes[nodei.Name], rti)
				}
			}
		}

		// Generate desired conditions
		newCond := v1alpha1.RoutingTableCondition{}
		t := metav1.NewTime(time.Now())
		if rti.Status.Conditions != nil {
			tmpDRTS.Conditions = rti.Status.Conditions
		} else {
			tmpDRTS.Conditions = make([]v1alpha1.RoutingTableCondition, 0)
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
					newCond.Message = "Failed reconciling on " + strings.Join(tmpDRTS.failedNodes, ",")
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

		_ = RTSetStatusCondition(&tmpDRTS.Conditions, newCond)

		desiredRTStatus[rti.Name] = *tmpDRTS
	}

	// Filling desiredNodeRoutingTables
	for nodeName, rtis := range affectedNodes {
		for _, rti := range rtis {
			var tmpNRTS desiredNRTInfo
			tmpNRTS.Name = rti.Name + "-" + generateShortHash(rti.Name+"#"+nodeName)
			tmpNRTS.NodeName = nodeName
			tmpNRTS.OwnerRTName = rti.Name
			tmpNRTS.OwnerRTUID = rti.UID
			tmpNRTS.IPRoutingTableID = rti.Status.IPRoutingTableID
			tmpNRTS.Routes = rti.Routes
			desiredNodeRoutingTables = append(desiredNodeRoutingTables, tmpNRTS)
		}
	}
	// Sort desiredNodeRoutingTables
	sort.SliceStable(desiredNodeRoutingTables, func(i, j int) bool {
		return desiredNodeRoutingTables[i].Name < desiredNodeRoutingTables[j].Name
	})
	// Set desiredNodeRoutingTables to internal values (for helm)
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
			GroupVersion,
			RTKind,
			"",
			rtName,
			object_patch.WithSubresource("/status"),
		)
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

func RTSetStatusCondition(conditions *[]v1alpha1.RoutingTableCondition, newCondition v1alpha1.RoutingTableCondition) (changed bool) {
	if conditions == nil {
		return false
	}

	timeNow := metav1.NewTime(time.Now())

	existingCondition := RTFindStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = timeNow
		}
		if newCondition.LastHeartbeatTime.IsZero() {
			newCondition.LastHeartbeatTime = timeNow
		}
		*conditions = append(*conditions, newCondition)
		return true
	}

	if !newCondition.LastHeartbeatTime.IsZero() {
		existingCondition.LastHeartbeatTime = newCondition.LastHeartbeatTime
	} else {
		existingCondition.LastHeartbeatTime = timeNow
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = timeNow
		}
		changed = true
	}

	if existingCondition.Reason != newCondition.Reason {
		existingCondition.Reason = newCondition.Reason
		changed = true
	}
	if existingCondition.Message != newCondition.Message {
		existingCondition.Message = newCondition.Message
		changed = true
	}
	return changed
}

func RTFindStatusCondition(conditions []v1alpha1.RoutingTableCondition, conditionType string) *v1alpha1.RoutingTableCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func NRTFindStatusCondition(conditions []v1alpha1.NodeRoutingTableCondition, conditionType string) *v1alpha1.NodeRoutingTableCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func deleteFinalizerFromNRT(input *go_hook.HookInput, nrtName string) {
	input.PatchCollector.Filter(
		func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var nrt v1alpha1.NodeRoutingTable
			err := sdk.FromUnstructured(obj, &nrt)
			if err != nil {
				input.LogEntry.Warnf("can't get NRT %v, error: %v", nrtName, err)
			}
			tmpNRTFinalizers := make([]string, 0)
			for _, fnlzr := range nrt.Finalizers {
				if fnlzr != finalizer {
					tmpNRTFinalizers = append(tmpNRTFinalizers, fnlzr)
				}
			}
			nrt.Finalizers = tmpNRTFinalizers
			return sdk.ToUnstructured(&nrt)
		},
		GroupVersion,
		NRTKind,
		"",
		nrtName,
	)
}
