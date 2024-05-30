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

package controller

import (
	"context"
	"fmt"
	"net"
	"static-routing-manager-agent/api/v1alpha1"
	"static-routing-manager-agent/pkg/config"
	"static-routing-manager-agent/pkg/logger"
	"static-routing-manager-agent/pkg/utils"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/vishvananda/netlink"

	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ipRuleCtrlName = "ip-rules-controller"
)

// Main

func RunIPRulesReconcilerAgentController(
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(ipRuleCtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.Debug(fmt.Sprintf("[NIRSReconciler] Received a reconcile.Request for CR %v", request.Name))

			nirs := &v1alpha1.NodeIPRuleSet{}
			err := cl.Get(ctx, request.NamespacedName, nirs)
			if err != nil && !errors2.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] Unable to get NodeIPRuleSet, name: %s", request.Name))
				return reconcile.Result{}, err
			}
			if nirs.Name == "" {
				log.Info(fmt.Sprintf("[NIRSReconciler] Seems like the NodeIPRuleSet for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}
			labelSelectorSet := map[string]string{v1alpha1.NodeNameLabel: cfg.NodeName}
			validatedSelector, _ := labels.ValidatedSelectorFromSet(labelSelectorSet)
			if !validatedSelector.Matches(labels.Set(nirs.Labels)) {
				log.Debug(fmt.Sprintf("[NIRSReconciler] This request is not intended(by label) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}
			if nirs.Spec.NodeName != cfg.NodeName {
				log.Debug(fmt.Sprintf("[NIRSReconciler] This request is not intended(by spec.nodeName) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}

			if nirs.Generation == nirs.Status.ObservedGeneration && nirs.DeletionTimestamp == nil {
				cond := utils.FindStatusCondition(nirs.Status.Conditions, v1alpha1.ReconciliationSucceedType)
				if cond != nil && cond.Status == metav1.ConditionTrue {
					log.Debug(fmt.Sprintf("[NIRSReconciler] There's nothing to do"))
					return reconcile.Result{}, nil
				}
			}
			log.Debug(fmt.Sprintf("[NIRSReconciler] NodeIPRuleSet %v needs to be reconciled. Set status to Pending", nirs.Name))
			tmpNIRS := new(v1alpha1.NodeIPRuleSet)
			*tmpNIRS = *nirs

			if nirs.Generation != nirs.Status.ObservedGeneration {
				err = utils.SetStatusConditionPendingToNIRS(ctx, cl, log, tmpNIRS)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NIRSReconciler] Unable to set status to Pending for NIRS %v", nirs.Name))
				}
			}

			// ============================= main logic start =============================
			log.Debug(fmt.Sprintf("[NIRSReconciler] Starts of the reconciliation (initiated by the k8s-event)"))
			shouldRequeue, err := runEventIPRuleReconcile(ctx, cl, log, cfg.NodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] An error occurred while route reconcile"))
			}

			if shouldRequeue {
				log.Warning(fmt.Sprintf("[NIRSReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}
			// ============================= main logic end =============================

			log.Debug(fmt.Sprintf("[NIRSReconciler] End of the reconciliation (initiated by the k8s-event)"))
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunIPRulesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.NodeIPRuleSet{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunIPRulesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	// trigger reconcile every 30 sec
	ctx := context.Background()
	go periodicalRunEventIPRuleReconcile(ctx, cfg, cl, log, cfg.NodeName)

	return c, nil
}

func runEventIPRuleReconcile(
	ctx context.Context,
	cl client.Client,
	log logger.Logger,
	nodeName string) (bool, error) {
	// Declaring variables
	var err error
	globalDesiredIPRulesForNode := make(IPRuleEntryMap)
	actualIPRulesOnNode := make(IPRuleEntryMap)
	nirsMap := nirsMapInit()

	// Getting all the NodeIPRuleSet associated with our node
	nirsList := &v1alpha1.NodeIPRuleSetList{}
	err = cl.List(ctx, nirsList, client.MatchingLabels{v1alpha1.NodeNameLabel: nodeName})
	if err != nil && !errors2.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("[NIRSReconciler] unable to list NodeIPRuleSet for node %s", nodeName))
		return true, err
	}

	// Getting all IPRules from our node
	log.Debug(fmt.Sprintf("[NIRSReconciler] Getting all IPRules from our node"))
	actualIPRulesOnNode, err = getActualIPRuleEntryMapFromNode()
	if err != nil {
		log.Error(err, fmt.Sprintf("[NIRSReconciler] unable to get Actual IPRules from node"))
		return true, err
	}
	if len(actualIPRulesOnNode) == 0 {
		log.Debug(fmt.Sprintf("[NIRSReconciler] There are no IPRules with Realm=" + strconv.Itoa(v1alpha1.D8Realm)))
	}

	for _, nirs := range nirsList.Items {
		nirsSummary := nirsSummaryInit()
		// Gathering facts
		log.Debug(fmt.Sprintf("[NIRSReconciler] Starting gather facts about nirs %v", nirs.Name))
		if nirsSummary.discoverFacts(nirs, &globalDesiredIPRulesForNode, &actualIPRulesOnNode, log) {
			(*nirsMap)[nirs.Name] = nirsSummary
			continue
		}

		// Actions: add IPRules
		if len(nirsSummary.desiredIPRulesToAddByNIRS) > 0 {
			log.Debug(fmt.Sprintf("[NIRSReconciler] Starting adding IPRules to the node"))
			nirsSummary.addIPRules(&actualIPRulesOnNode, log)
		}

		(*nirsMap)[nirs.Name] = nirsSummary
	}

	// Actions: delete IPRules and finalizers
	nirsMap.deleteIPRulesAndFinalizers(globalDesiredIPRulesForNode, actualIPRulesOnNode, log)

	// Generate new condition for each processed nirs
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting generate new conditions"))
	shouldRequeue := nirsMap.generateNewCondition()

	// Update state in k8s for each processed nirs
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting updating resourses in k8s"))
	nirsMap.updateStateInK8S(ctx, cl, log)

	return shouldRequeue, nil
}

func periodicalRunEventIPRuleReconcile(
	ctx context.Context,
	cfg config.Options,
	cl client.Client,
	log logger.Logger,
	nodeName string,
) {
	ticker := time.NewTicker(cfg.PeriodicReconciliationInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug(fmt.Sprintf("[NIRSReconciler] Starts a periodic reconciliation (initiated by a timer)"))
			_, err := runEventIPRuleReconcile(ctx, cl, log, nodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] an error occurred while IPRule reconcile"))
			}
			log.Debug(fmt.Sprintf("[NIRSReconciler] Ends a periodic reconciliation (initiated by a timer)"))
		case <-ctx.Done():
			log.Debug(fmt.Sprintf("[NIRSReconciler] Completion of periodic reconciliations"))
			return
		}
	}
}

// IPRuleEntry: type, service functions and methods

type IPRuleEntry struct {
	Priority   int
	Invert     bool
	Src        string
	Dst        string
	IPProto    int
	SPortRange *v1alpha1.PortRange
	DPortRange *v1alpha1.PortRange
	Tos        string
	FWMark     string
	IifName    string
	OifName    string
	UIDRange   *v1alpha1.UIDRange
	Table      int
}

func (ire *IPRuleEntry) getHash() string {
	hashRaw := make([]string, 0)
	hashRaw = append(hashRaw, strconv.Itoa(ire.Priority))
	hashRaw = append(hashRaw, strconv.FormatBool(ire.Invert))
	hashRaw = append(hashRaw, ire.Src)
	hashRaw = append(hashRaw, ire.Dst)
	hashRaw = append(hashRaw, strconv.Itoa(ire.IPProto))
	if ire.SPortRange != nil {
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.SPortRange.Start), 10))
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.SPortRange.End), 10))
	}
	if ire.DPortRange != nil {
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.DPortRange.Start), 10))
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.DPortRange.End), 10))
	}
	hashRaw = append(hashRaw, ire.Tos)
	hashRaw = append(hashRaw, ire.FWMark)
	hashRaw = append(hashRaw, ire.IifName)
	hashRaw = append(hashRaw, ire.OifName)
	if ire.UIDRange != nil {
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.UIDRange.Start), 10))
		hashRaw = append(hashRaw, strconv.FormatUint(uint64(ire.UIDRange.End), 10))
	}
	hashRaw = append(hashRaw, strconv.Itoa(ire.Table))
	return strings.Join(hashRaw, "#")
}

func (ire *IPRuleEntry) getNetlinkRule() (*netlink.Rule, error) {
	// Prepare rule for netlink

	PreparedIPRule := netlink.NewRule()

	if ire.Priority > 0 {
		PreparedIPRule.Priority = ire.Priority
	}
	if ire.Table > 0 {
		PreparedIPRule.Table = ire.Table
	}
	if ire.FWMark != "" {
		FWMarkRaw := strings.Split(ire.FWMark, "/")
		markRaw := FWMarkRaw[0]
		markRaw = markRaw[2:]
		mark, err := strconv.ParseInt(markRaw, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("unable to parse filed FWMARK in rule %v, err: %w",
				*ire,
				err,
			)
		}
		PreparedIPRule.Mark = int(mark)
		if len(FWMarkRaw) > 1 {
			maskRaw := FWMarkRaw[1]
			maskRaw = maskRaw[2:]
			mask, err := strconv.ParseInt(maskRaw, 16, 32)
			if err != nil {
				return nil, fmt.Errorf("unable to parse filed FWMASK in rule %v, err: %w",
					*ire,
					err,
				)
			}
			PreparedIPRule.Mask = int(mask)
		}
	}
	if ire.Tos != "" {
		tosRaw := ire.Tos[2:]
		tos, err := strconv.ParseInt(tosRaw, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("unable to parse filed TOS in rule %v, err: %w",
				*ire,
				err,
			)
		}
		PreparedIPRule.Tos = uint(tos)
	}
	if ire.Src != "" {
		ip, src, err := net.ParseCIDR(ire.Src)
		if err != nil {
			return nil, fmt.Errorf("unable to parse filed FROM in rule %v, err: %w",
				*ire,
				err,
			)
		}
		if !ip.Equal(src.IP) {
			return nil, fmt.Errorf("rule %v is incorrect, filed FROM is not a valid network address. perhaps %v was meant",
				*ire,
				src.String(),
			)
		}
		PreparedIPRule.Src = src
	}
	if ire.Dst != "" {
		ip, dst, err := net.ParseCIDR(ire.Dst)
		if err != nil {
			return nil, fmt.Errorf("unable to parse filed TO in rule %v, err: %w",
				*ire,
				err,
			)
		}
		if !ip.Equal(dst.IP) {
			return nil, fmt.Errorf("rule %v is incorrect, filed TO is not a valid network address. perhaps %v was meant",
				*ire,
				dst.String(),
			)
		}
		PreparedIPRule.Dst = dst
	}
	if ire.IifName != "" {
		PreparedIPRule.IifName = ire.IifName
	}
	if ire.OifName != "" {
		PreparedIPRule.OifName = ire.OifName
	}
	if ire.Invert == true {
		PreparedIPRule.Invert = ire.Invert
	}
	if ire.IPProto > 0 {
		PreparedIPRule.IPProto = ire.IPProto
	}
	if ire.SPortRange != nil {
		PreparedIPRule.Sport = netlink.NewRulePortRange(ire.SPortRange.Start, ire.SPortRange.End)
	}
	if ire.DPortRange != nil {
		PreparedIPRule.Dport = netlink.NewRulePortRange(ire.DPortRange.Start, ire.DPortRange.End)
	}
	if ire.UIDRange != nil {
		PreparedIPRule.UIDRange = netlink.NewRuleUIDRange(ire.UIDRange.Start, ire.UIDRange.End)
	}

	return PreparedIPRule, nil
}

func getIPRuleEntryFromNetlinkRule(nlRule netlink.Rule) IPRuleEntry {
	PreparedIPRule := IPRuleEntry{}

	if nlRule.Priority > 0 {
		PreparedIPRule.Priority = nlRule.Priority
	}
	if nlRule.Table > 0 {
		PreparedIPRule.Table = nlRule.Table
	}
	if nlRule.Mark != -1 {
		FWMark := fmt.Sprintf("0x%x", nlRule.Mark)
		if nlRule.Mask != -1 {
			FWMark = fmt.Sprintf("%s/0x%x", FWMark, nlRule.Mask)
		}
		PreparedIPRule.FWMark = FWMark
	}
	if nlRule.Tos > 0 {
		PreparedIPRule.Tos = fmt.Sprintf("0x%x", nlRule.Tos)
	}
	if nlRule.Src != nil {
		PreparedIPRule.Src = nlRule.Src.String()
	}
	if nlRule.Dst != nil {
		PreparedIPRule.Dst = nlRule.Dst.String()
	}
	if nlRule.IifName != "" {
		PreparedIPRule.IifName = nlRule.IifName
	}
	if nlRule.OifName != "" {
		PreparedIPRule.OifName = nlRule.OifName
	}
	if nlRule.Invert == true {
		PreparedIPRule.Invert = nlRule.Invert
	}
	if nlRule.IPProto > 0 {
		PreparedIPRule.IPProto = nlRule.IPProto
	}
	if nlRule.Dport != nil {
		PreparedIPRule.DPortRange = &v1alpha1.PortRange{
			Start: nlRule.Dport.Start,
			End:   nlRule.Dport.End,
		}
	}
	if nlRule.Sport != nil {
		PreparedIPRule.SPortRange = &v1alpha1.PortRange{
			Start: nlRule.Sport.Start,
			End:   nlRule.Sport.End,
		}
	}
	if nlRule.UIDRange != nil {
		PreparedIPRule.UIDRange = &v1alpha1.UIDRange{
			Start: nlRule.UIDRange.Start,
			End:   nlRule.UIDRange.End,
		}
	}

	return PreparedIPRule
}

// IPRuleEntryMap: type, service functions and methods

type IPRuleEntryMap map[string]IPRuleEntry

func (irem *IPRuleEntryMap) AppendIRE(ire IPRuleEntry) {
	if len(*irem) == 0 {
		*irem = make(map[string]IPRuleEntry)
	}
	(*irem)[ire.getHash()] = ire
}

func (irem *IPRuleEntryMap) AppendIR(ipRule v1alpha1.IPRule) {
	if len(*irem) == 0 {
		*irem = make(map[string]IPRuleEntry)
	}

	for _, from := range ipRule.Selectors.From {
		for _, to := range ipRule.Selectors.To {
			ire := IPRuleEntry{
				Priority: ipRule.Priority,
				Invert:   ipRule.Selectors.Not,
				IPProto:  ipRule.Selectors.IPProto,
				Tos:      ipRule.Selectors.Tos,
				FWMark:   ipRule.Selectors.FWMark,
				IifName:  ipRule.Selectors.IIf,
				OifName:  ipRule.Selectors.OIf,
				Table:    ipRule.Actions.Lookup.IPRoutingTableID,
			}
			ire.SPortRange = &v1alpha1.PortRange{
				Start: ipRule.Selectors.SPortRange.Start,
			}
			if ipRule.Selectors.SPortRange.End == 0 &&
				ipRule.Selectors.SPortRange.End != ipRule.Selectors.SPortRange.Start {
				ire.SPortRange.End = ipRule.Selectors.SPortRange.Start
			} else {
				ire.SPortRange.End = ipRule.Selectors.SPortRange.End
			}

			ire.DPortRange = &v1alpha1.PortRange{
				Start: ipRule.Selectors.DPortRange.Start,
			}
			if ipRule.Selectors.DPortRange.End == 0 &&
				ipRule.Selectors.DPortRange.End != ipRule.Selectors.DPortRange.Start {
				ire.DPortRange.End = ipRule.Selectors.DPortRange.Start
			} else {
				ire.DPortRange.End = ipRule.Selectors.DPortRange.End
			}

			ire.UIDRange = &v1alpha1.UIDRange{
				Start: ipRule.Selectors.UIDRange.Start,
			}
			if ipRule.Selectors.UIDRange.End == 0 &&
				ipRule.Selectors.UIDRange.End != ipRule.Selectors.UIDRange.Start {
				ire.UIDRange.End = ipRule.Selectors.UIDRange.Start
			} else {
				ire.UIDRange.End = ipRule.Selectors.UIDRange.End
			}

			ire.Src = from
			ire.Dst = to

			(*irem)[ire.getHash()] = ire
		}
	}
}

// nirsSummary: type, service functions and methods

type nirsSummary struct {
	k8sResources              *v1alpha1.NodeIPRuleSet
	newReconciliationStatus   utils.ReconciliationStatus
	desiredIPRulesByNIRS      IPRuleEntryMap
	lastAppliedIPRulesByNIRS  IPRuleEntryMap
	desiredIPRulesToAddByNIRS []IPRuleEntry
	desiredIPRulesToDelByNIRS IPRuleEntryMap
	nirsWasDeleted            bool
	specNeedToUpdate          bool
}

func nirsSummaryInit() *nirsSummary {
	return &nirsSummary{
		k8sResources:              new(v1alpha1.NodeIPRuleSet),
		newReconciliationStatus:   utils.ReconciliationStatus{},
		desiredIPRulesByNIRS:      IPRuleEntryMap{},
		lastAppliedIPRulesByNIRS:  IPRuleEntryMap{},
		desiredIPRulesToAddByNIRS: make([]IPRuleEntry, 0),
		desiredIPRulesToDelByNIRS: IPRuleEntryMap{},
		nirsWasDeleted:            false,
		specNeedToUpdate:          false,
	}
}

func (ns *nirsSummary) discoverFacts(nirs v1alpha1.NodeIPRuleSet, globalDesiredIPRulesForNode, actualIPRulesOnNode *IPRuleEntryMap, log logger.Logger) bool {
	// Filling nirsK8sResourcesMap[nirs.Name] and nirsReconciliationStatusMap[nirs.Name]
	tmpNIRS := nirs
	tmpNIRS.Status.ObservedGeneration = nirs.Generation
	ns.k8sResources = &tmpNIRS
	ns.newReconciliationStatus = utils.ReconciliationStatus{IsSuccess: true}
	ns.specNeedToUpdate = false

	// If NIRS was deleted filling map desiredIPRulesToDelByNIRS and set flag nirsWasDeleted
	if nirs.DeletionTimestamp != nil {
		log.Debug(fmt.Sprintf("[NIRSReconciler] NIRS %v is marked for deletion", nirs.Name))
		log.Debug(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToDel, and set flag nirsWasDeleted "))
		tmpIREM := make(IPRuleEntryMap)
		for _, ipRule := range nirs.Spec.IPRules {
			tmpIREM.AppendIR(ipRule)
		}
		ns.desiredIPRulesToDelByNIRS = tmpIREM
		ns.nirsWasDeleted = true
		return true
	}

	// Filling desiredIPRulesByNRT and globalDesiredIPRulesForNode
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting filling maps: desiredIPRules and globalDesiredIPRules"))
	for _, ipRule := range nirs.Spec.IPRules {
		ns.desiredIPRulesByNIRS.AppendIR(ipRule)
		globalDesiredIPRulesForNode.AppendIR(ipRule)
	}

	// Filling lastAppliedRoutesByNRT
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting filling map lastAppliedIPRules"))
	if nirs.Status.AppliedIPRules != nil {
		for _, ipRule := range nirs.Status.AppliedIPRules {
			ns.lastAppliedIPRulesByNIRS.AppendIR(ipRule)
		}
	}

	// Filling desiredIPRulesToAddByNIRS
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToAdd"))
	for hash, desiredIPRule := range ns.desiredIPRulesByNIRS {
		if _, ok := (*actualIPRulesOnNode)[hash]; !ok {
			ns.desiredIPRulesToAddByNIRS = append(ns.desiredIPRulesToAddByNIRS, desiredIPRule)
		}
	}

	// Filling desiredIPRulesToDelByNIRS
	log.Debug(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToDel"))
	tmpIREM := make(IPRuleEntryMap)
	for hash, ipRule := range ns.lastAppliedIPRulesByNIRS {
		if _, ok := ns.desiredIPRulesByNIRS[hash]; !ok {
			tmpIREM.AppendIRE(ipRule)
		}
	}
	ns.desiredIPRulesToDelByNIRS = tmpIREM

	return false
}

func (ns *nirsSummary) addIPRules(actualIPRulesOnNode *IPRuleEntryMap, log logger.Logger) {
	status := ns.newReconciliationStatus
	for _, ipRule := range ns.desiredIPRulesToAddByNIRS {
		log.Debug(fmt.Sprintf("IPRule %v should be added", ipRule))
		if _, ok := (*actualIPRulesOnNode)[ipRule.getHash()]; ok {
			log.Debug(fmt.Sprintf("but it is already present on Node"))
			continue
		}
		err := addIPRuleToNode(ipRule)
		if err == nil {
			actualIPRulesOnNode.AppendIRE(ipRule)
		} else {
			log.Debug(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	ns.newReconciliationStatus = status
}

// nirsMap: type, service functions and methods

type nirsMap map[string]*nirsSummary

func nirsMapInit() *nirsMap {
	newNIRSMap := new(nirsMap)
	*newNIRSMap = make(map[string]*nirsSummary)
	return newNIRSMap
}

func (nm *nirsMap) deleteIPRulesAndFinalizers(globalDesiredIPRulesForNode, actualIPRulesOnNode IPRuleEntryMap, log logger.Logger) {
	for nirsName, ns := range *nm {
		if len(ns.desiredIPRulesToDelByNIRS) == 0 && !ns.nirsWasDeleted {
			log.Debug(fmt.Sprintf("[NIRSReconciler] NIRS %v has no entries in desiredIPRulesToDelByNIRS and DeletionTimestamp is not set", nirsName))
			continue
		}
		log.Debug(fmt.Sprintf("[NIRSReconciler] Starting to delete IPRules deleted from NIRS %v from node", nirsName))
		status := ns.newReconciliationStatus
		ns.newReconciliationStatus = deleteIPRuleEntriesFromNode(
			ns.desiredIPRulesToDelByNIRS,
			globalDesiredIPRulesForNode,
			actualIPRulesOnNode,
			status,
			log,
		)
		if ns.nirsWasDeleted && ns.newReconciliationStatus.IsSuccess {
			log.Debug(fmt.Sprintf("[NIRSReconciler] NIRS %v has been deleted and its IPRules has been successfully deleted too. Clearing the finalizer in NIRS", nirsName))
			removeFinalizerFromNIRS(ns.k8sResources)
			ns.specNeedToUpdate = true
		}
	}
}

func (nm *nirsMap) generateNewCondition() bool {
	shouldRequeue := false
	for _, ns := range *nm {
		newCond := v1alpha1.ExtendedCondition{}
		t := metav1.NewTime(time.Now())

		if ns.k8sResources.Status.Conditions == nil {
			ns.k8sResources.Status.Conditions = make([]v1alpha1.ExtendedCondition, 0)
		}

		if ns.newReconciliationStatus.IsSuccess {
			ns.k8sResources.Status.AppliedIPRules = ns.k8sResources.Spec.IPRules

			newCond.Type = v1alpha1.ReconciliationSucceedType
			newCond.LastHeartbeatTime = t
			newCond.Status = metav1.ConditionTrue
			newCond.Reason = v1alpha1.ReconciliationReasonSucceed
			newCond.Message = ""
		} else {
			newCond.Type = v1alpha1.ReconciliationSucceedType
			newCond.LastHeartbeatTime = t
			newCond.Status = metav1.ConditionFalse
			newCond.Reason = v1alpha1.ReconciliationReasonFailed
			newCond.Message = ns.newReconciliationStatus.ErrorMessage

			shouldRequeue = true
		}
		_ = utils.SetStatusCondition(&ns.k8sResources.Status.Conditions, newCond)
	}
	return shouldRequeue
}

func (nm *nirsMap) updateStateInK8S(ctx context.Context, cl client.Client, log logger.Logger) {
	var err error
	for nirsName, ns := range *nm {
		if ns.specNeedToUpdate && ns.k8sResources.DeletionTimestamp != nil {
			// Update spec if we need to remove the finalizer
			log.Debug(fmt.Sprintf("Update of NIRS: %v", nirsName))
			err = cl.Update(ctx, ns.k8sResources)
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to update CR NodeIPRuleSet %v, err: %v", nirsName, err))
			}
		}
		// Update status every time
		log.Debug(fmt.Sprintf("Update status of NIRS: %v", nirsName))
		err = cl.Status().Update(ctx, ns.k8sResources)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to update status for CR NodeIPRuleSet %v, err: %v", nirsName, err))
		}
	}
}

// netlink service functions

func addIPRuleToNode(ipRule IPRuleEntry) error {
	PreparedIPRule, err := ipRule.getNetlinkRule()
	if err != nil {
		return fmt.Errorf("unable to parse IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	PreparedIPRule.Flow = v1alpha1.D8Realm
	err = netlink.RuleAdd(PreparedIPRule)
	if err != nil {
		return fmt.Errorf("unable to add IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	return nil
}

func delIPRuleFromNode(ipRule IPRuleEntry) error {
	PreparedIPRule, err := ipRule.getNetlinkRule()
	if err != nil {
		return fmt.Errorf("unable to parse IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	PreparedIPRule.Flow = v1alpha1.D8Realm
	err = netlink.RuleDel(PreparedIPRule)
	if err != nil {
		return fmt.Errorf("unable to del IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	return nil
}

func getActualIPRuleEntryMapFromNode() (IPRuleEntryMap, error) {
	nlRules, err := netlink.RuleListFiltered(netlink.FAMILY_V4, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed get IPRule from node, err: %w", err)
	}
	airem := make(IPRuleEntryMap)

	for _, nlRule := range nlRules {
		if nlRule.Flow != v1alpha1.D8Realm {
			continue
		}
		ire := getIPRuleEntryFromNetlinkRule(nlRule)
		airem.AppendIRE(ire)
	}
	return airem, nil
}

// other service functions

func deleteIPRuleEntriesFromNode(delIREM, gdIREM, actIREM IPRuleEntryMap, status utils.ReconciliationStatus, log logger.Logger) utils.ReconciliationStatus {
	for hash, ipRule := range delIREM {
		log.Debug(fmt.Sprintf("IPRule %v should be deleted", ipRule))
		if _, ok := (gdIREM)[hash]; ok {
			log.Debug(fmt.Sprintf("but it is present in other NIRS"))
			continue
		}
		if _, ok := (actIREM)[hash]; !ok {
			log.Debug(fmt.Sprintf("but it is not present on Node"))
			continue
		}
		err := delIPRuleFromNode(ipRule)
		if err != nil {
			log.Debug(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	return status
}

func removeFinalizerFromNIRS(nirs *v1alpha1.NodeIPRuleSet) {
	var tmpNIRSFinalizers []string
	tmpNIRSFinalizers = []string{}
	for _, fnlzr := range nirs.Finalizers {
		if fnlzr != v1alpha1.Finalizer {
			tmpNIRSFinalizers = append(tmpNIRSFinalizers, fnlzr)
		}
	}
	nirs.Finalizers = tmpNIRSFinalizers
}
