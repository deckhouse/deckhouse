/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"sort"
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
	nirsKeyPath = "staticRoutingManager.internal.nodeIPRuleSets"
)

type IPRuleSetInfo struct {
	Name         string
	UID          types.UID
	Generation   int64
	IsDeleted    bool
	IPRules      []v1alpha1.IPRule
	NodeSelector map[string]string
	Status       v1alpha1.IPRuleSetStatus
}

type SDNInternalNodeIPRuleSetInfo struct {
	Name      string
	IsDeleted bool
	NodeName  string
	Ready     bool
	Reason    string
}

type RTLiteInfo struct {
	Name             string
	IPRoutingTableID int
}

type desiredNIRSInfo struct {
	Name         string            `json:"name"`
	NodeName     string            `json:"nodeName"`
	OwnerIRSName string            `json:"ownerIRSName"`
	OwnerIRSUID  types.UID         `json:"ownerIRSUID"`
	IPRules      []v1alpha1.IPRule `json:"rules"`
}

type irsStatusPlus struct {
	v1alpha1.IPRuleSetStatus
	failedNodes []string
	localErrors []string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/static-routing-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "iprulesets",
			ApiVersion: v1alpha1.GroupVersion,
			Kind:       v1alpha1.IRSKind,
			FilterFunc: applyIPRuleSetFilter,
		},
		{
			Name:       "nodeiprulesets",
			ApiVersion: v1alpha1.InternalGroupVersion,
			Kind:       v1alpha1.NIRSKind,
			FilterFunc: applyNodeIPRuleSetFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: lib.ApplyNodeFilter,
		},
		{
			Name:       "routingtables",
			ApiVersion: v1alpha1.GroupVersion,
			Kind:       v1alpha1.RTKind,
			FilterFunc: applyRTLiteFilter,
		},
	},
}, ipRuleSetsHandler)

func applyIPRuleSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		irs    v1alpha1.IPRuleSet
		result IPRuleSetInfo
	)

	err := sdk.FromUnstructured(obj, &irs)
	if err != nil {
		return nil, err
	}

	result = IPRuleSetInfo{
		Name:         irs.Name,
		UID:          irs.UID,
		Generation:   irs.Generation,
		IsDeleted:    irs.DeletionTimestamp != nil,
		IPRules:      irs.Spec.IPRules,
		NodeSelector: irs.Spec.NodeSelector,
		Status:       irs.Status,
	}

	return result, nil
}

func applyNodeIPRuleSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		nirs   v1alpha1.SDNInternalNodeIPRuleSet
		result SDNInternalNodeIPRuleSetInfo
	)
	err := sdk.FromUnstructured(obj, &nirs)
	if err != nil {
		return nil, err
	}

	result.Name = nirs.Name
	result.IsDeleted = nirs.DeletionTimestamp != nil
	result.NodeName = nirs.Spec.NodeName

	cond := lib.FindStatusCondition(nirs.Status.Conditions, v1alpha1.ReconciliationSucceedType)
	if cond == nil {
		result.Ready = false
		result.Reason = v1alpha1.ReconciliationReasonPending
	} else {
		result.Ready = cond.Status == metav1.ConditionTrue
		result.Reason = cond.Reason
	}

	return result, nil
}

func applyRTLiteFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		rt     v1alpha1.RoutingTable
		result RTLiteInfo
	)

	err := sdk.FromUnstructured(obj, &rt)
	if err != nil {
		return nil, err
	}

	result = RTLiteInfo{
		Name:             rt.Name,
		IPRoutingTableID: rt.Status.IPRoutingTableID,
	}

	return result, nil
}

func ipRuleSetsHandler(input *go_hook.HookInput) error {
	// Init vars

	actualNodeIPRuleSets := make(map[string]SDNInternalNodeIPRuleSetInfo)
	allNodes := make(map[string]struct{})
	allRTs := make(map[string]int)
	affectedNodes := make(map[string][]IPRuleSetInfo)
	desiredIRSStatuses := make(map[string]irsStatusPlus)
	desiredNodeIPRuleSets := make([]desiredNIRSInfo, 0)

	// Filling allNodes
	for _, nodeRaw := range input.Snapshots["nodes"] {
		node := nodeRaw.(lib.NodeInfo)
		allNodes[node.Name] = struct{}{}
	}

	// Filling allRTs
	for _, rtRaw := range input.Snapshots["routingtables"] {
		rt := rtRaw.(RTLiteInfo)
		if rt.IPRoutingTableID != 0 {
			allRTs[rt.Name] = rt.IPRoutingTableID
		}
	}

	// Filling actualNodeIPRuleSets and delete finalizers from orphan NIRSs
	for _, nirsRaw := range input.Snapshots["nodeiprulesets"] {
		nirs := nirsRaw.(SDNInternalNodeIPRuleSetInfo)
		actualNodeIPRuleSets[nirs.Name] = nirs
		if _, ok := allNodes[nirs.NodeName]; !ok && nirs.IsDeleted {
			input.LogEntry.Infof("An orphan NIRS %v was found. It will be deleted", nirs.Name)
			lib.DeleteFinalizer(
				input,
				nirs.Name,
				v1alpha1.InternalGroupVersion,
				v1alpha1.NIRSKind,
				v1alpha1.Finalizer,
			)
		}
	}

	// main loop
	for _, irsiRaw := range input.Snapshots["iprulesets"] {
		irsi := irsiRaw.(IPRuleSetInfo)

		// DIRS stands for Desired IP Rule Set
		tmpDIRSStatus := new(irsStatusPlus)
		tmpDIRSStatus.failedNodes = make([]string, 0)
		tmpDIRSStatus.localErrors = make([]string, 0)

		if _, ok := desiredIRSStatuses[irsi.Name]; ok {
			*tmpDIRSStatus = desiredIRSStatuses[irsi.Name]
		}

		// Generate desired ObservedGeneration
		tmpDIRSStatus.ObservedGeneration = irsi.Generation

		// If IPRoutingTableID is empty in Rule then get it from RoutingTable (for each)
		for i, iprule := range irsi.IPRules {
			if iprule.Actions.Lookup.IPRoutingTableID != 0 {
				continue
			}
			if iprule.Actions.Lookup.RoutingTableName == "" {
				errr := fmt.Sprintf("can't get RoutingTableID in IPRuleSet %v for rule %v", irsi.Name, irsi.IPRules)
				input.LogEntry.Warnf(errr)
				tmpDIRSStatus.localErrors = append(tmpDIRSStatus.localErrors, errr)
				continue
			}
			rtName := iprule.Actions.Lookup.RoutingTableName
			if rtID, ok := allRTs[rtName]; ok {
				irsi.IPRules[i].Actions.Lookup.IPRoutingTableID = rtID
			} else {
				errr := fmt.Sprintf("can't get RoutingTableID in IPRuleSet %v for rule %v", irsi.Name, irsi.IPRules)
				input.LogEntry.Warnf(errr)
				tmpDIRSStatus.localErrors = append(tmpDIRSStatus.localErrors, errr)
			}
		}

		// Generate desired AffectedNodeIPRuleSets and ReadyNodeIPRuleSets, and filling affectedNodes
		validatedSelector, _ := labels.ValidatedSelectorFromSet(irsi.NodeSelector)
		for _, nodeiRaw := range input.Snapshots["nodes"] {
			nodei := nodeiRaw.(lib.NodeInfo)

			if validatedSelector.Matches(labels.Set(nodei.Labels)) {
				tmpDIRSStatus.AffectedNodeIPRuleSets++
				nirsName := irsi.Name + "-" + lib.GenerateShortHash(irsi.Name+"#"+nodei.Name)
				if _, ok := actualNodeIPRuleSets[nirsName]; ok {
					if actualNodeIPRuleSets[nirsName].Ready {
						tmpDIRSStatus.ReadyNodeIPRuleSets++
					} else if actualNodeIPRuleSets[nirsName].Reason == v1alpha1.ReconciliationReasonFailed {
						tmpDIRSStatus.failedNodes = append(tmpDIRSStatus.failedNodes, nodei.Name)
					}
				}

				// Filling affectedNodes
				affectedNodes[nodei.Name] = append(affectedNodes[nodei.Name], irsi)
			}
		}

		// Generate desired conditions
		newCond := v1alpha1.ExtendedCondition{}
		t := metav1.NewTime(time.Now())
		if irsi.Status.Conditions != nil {
			tmpDIRSStatus.Conditions = irsi.Status.Conditions
		} else {
			tmpDIRSStatus.Conditions = make([]v1alpha1.ExtendedCondition, 0)
		}

		if len(tmpDIRSStatus.localErrors) == 0 {
			if tmpDIRSStatus.ReadyNodeIPRuleSets == tmpDIRSStatus.AffectedNodeIPRuleSets {
				newCond.Type = v1alpha1.ReconciliationSucceedType
				newCond.LastHeartbeatTime = t
				newCond.Status = metav1.ConditionTrue
				newCond.Reason = v1alpha1.ReconciliationReasonSucceed
				newCond.Message = ""
			} else {
				if len(tmpDIRSStatus.failedNodes) > 0 {
					newCond.Type = v1alpha1.ReconciliationSucceedType
					newCond.LastHeartbeatTime = t
					newCond.Status = metav1.ConditionFalse
					newCond.Reason = v1alpha1.ReconciliationReasonFailed
					newCond.Message = "Failed reconciling on " + strings.Join(tmpDIRSStatus.failedNodes, ", ")
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
			newCond.Message = strings.Join(tmpDIRSStatus.localErrors, "\n")
		}

		_ = lib.SetStatusCondition(&tmpDIRSStatus.Conditions, newCond)

		desiredIRSStatuses[irsi.Name] = *tmpDIRSStatus
	}

	// Filling desiredNodeIPRuleSets
	for nodeName, irsis := range affectedNodes {
		for _, irsi := range irsis {
			var tmpNIRSSpec desiredNIRSInfo
			tmpNIRSSpec.Name = irsi.Name + "-" + lib.GenerateShortHash(irsi.Name+"#"+nodeName)
			tmpNIRSSpec.NodeName = nodeName
			tmpNIRSSpec.OwnerIRSName = irsi.Name
			tmpNIRSSpec.OwnerIRSUID = irsi.UID

			tmpNIRSSpec.IPRules = make([]v1alpha1.IPRule, 0)
			for _, iprule := range irsi.IPRules {
				if iprule.Actions.Lookup.IPRoutingTableID != 0 {
					tmpNIRSSpec.IPRules = append(tmpNIRSSpec.IPRules, iprule)
				}
			}

			if len(tmpNIRSSpec.IPRules) > 0 {
				desiredNodeIPRuleSets = append(desiredNodeIPRuleSets, tmpNIRSSpec)
			}
		}
	}
	// Sort desiredNodeIPRuleSets to prevent helm flapping
	sort.SliceStable(desiredNodeIPRuleSets, func(i, j int) bool {
		return desiredNodeIPRuleSets[i].Name < desiredNodeIPRuleSets[j].Name
	})
	input.Values.Set(nirsKeyPath, desiredNodeIPRuleSets)

	// Update status in k8s
	for irsName, dIRSStatus := range desiredIRSStatuses {
		newStatus := v1alpha1.IPRuleSetStatus{}

		newStatus.ObservedGeneration = dIRSStatus.ObservedGeneration
		newStatus.ReadyNodeIPRuleSets = dIRSStatus.ReadyNodeIPRuleSets
		newStatus.AffectedNodeIPRuleSets = dIRSStatus.AffectedNodeIPRuleSets
		newStatus.Conditions = dIRSStatus.Conditions

		statusPatch := map[string]interface{}{
			"status": newStatus,
		}

		input.PatchCollector.MergePatch(
			statusPatch,
			v1alpha1.GroupVersion,
			v1alpha1.IRSKind,
			"",
			irsName,
			object_patch.WithSubresource("/status"),
		)
	}

	return nil
}
