/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/api/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/config"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/pkg/utils"

	"github.com/go-logr/logr"

	"github.com/mitchellh/hashstructure/v2"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

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
	nh netns.NsHandle,
	log logr.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(ipRuleCtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Received a reconcile.Request for CR %v", request.Name))

			nirs := &v1alpha1.SDNInternalNodeIPRuleSet{}
			err := cl.Get(ctx, request.NamespacedName, nirs)
			if err != nil && !k8serrors.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] Unable to get SDNInternalNodeIPRuleSet, name: %s", request.Name))
				return reconcile.Result{}, err
			}
			if nirs.Name == "" {
				log.Info(fmt.Sprintf("[NIRSReconciler] Seems like the SDNInternalNodeIPRuleSet for the request %s was deleted. Reconcile retrying will stop.", request.Name))
				return reconcile.Result{}, nil
			}
			labelSelectorSet := map[string]string{v1alpha1.NodeNameLabel: cfg.NodeName}
			validatedSelector, _ := labels.ValidatedSelectorFromSet(labelSelectorSet)
			if !validatedSelector.Matches(labels.Set(nirs.Labels)) {
				log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] This request is not intended(by label) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}
			if nirs.Spec.NodeName != cfg.NodeName {
				log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] This request is not intended(by spec.nodeName) for our node (%v)", cfg.NodeName))
				return reconcile.Result{}, nil
			}

			if nirs.Generation == nirs.Status.ObservedGeneration && nirs.DeletionTimestamp == nil {
				cond := utils.FindStatusCondition(nirs.Status.Conditions, v1alpha1.ReconciliationSucceedType)
				if cond != nil && cond.Status == metav1.ConditionTrue {
					log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] There's nothing to do"))
					return reconcile.Result{}, nil
				}
			}
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] SDNInternalNodeIPRuleSet %v needs to be reconciled. Set status to Pending", nirs.Name))
			tmpNIRS := new(v1alpha1.SDNInternalNodeIPRuleSet)
			*tmpNIRS = *nirs

			if nirs.Generation != nirs.Status.ObservedGeneration {
				err = utils.SetStatusConditionPendingToNIRS(ctx, cl, log, tmpNIRS)
				if err != nil {
					log.Error(err, fmt.Sprintf("[NIRSReconciler] Unable to set status to Pending for NIRS %v", nirs.Name))
				}
			}

			// ============================= main logic start =============================
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starts of the reconciliation (initiated by the k8s-event)"))
			shouldRequeue, err := runEventIPRuleReconcile(ctx, cl, nh, log, cfg.NodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] An error occurred while route reconcile"))
			}

			if shouldRequeue {
				log.V(config.WarnLvl).Info(fmt.Sprintf("[NIRSReconciler] Reconciler will requeue the request, name: %s", request.Name))
				return reconcile.Result{
					RequeueAfter: cfg.RequeueInterval * time.Second,
				}, nil
			}
			// ============================= main logic end =============================

			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] End of the reconciliation (initiated by the k8s-event)"))
			return reconcile.Result{}, nil
		}),
	})
	if err != nil {
		log.Error(err, "[RunIPRulesReconcilerAgentController] unable to create controller")
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.SDNInternalNodeIPRuleSet{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Error(err, "[RunIPRulesReconcilerAgentController] unable to watch the events")
		return nil, err
	}

	// trigger reconcile every 30 sec
	ctx := context.Background()
	go periodicalRunEventIPRuleReconcile(ctx, cfg, cl, nh, log, cfg.NodeName)

	return c, nil
}

func runEventIPRuleReconcile(
	ctx context.Context,
	cl client.Client,
	nh netns.NsHandle,
	log logr.Logger,
	nodeName string) (bool, error) {
	// Declaring variables
	var err error
	globalDesiredIPRulesForNode := make(IPRuleEntryMap)
	actualIPRulesOnNode := make(IPRuleEntryMap)
	nirsMap := nirsMapInit()

	// Getting all the SDNInternalNodeIPRuleSet associated with our node
	nirsList := &v1alpha1.SDNInternalNodeIPRuleSetList{}
	err = cl.List(ctx, nirsList, client.MatchingLabels{v1alpha1.NodeNameLabel: nodeName})
	if err != nil && !k8serrors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("[NIRSReconciler] unable to list NodeIPRuleSet for node %s", nodeName))
		return true, err
	}

	// Getting all IPRules from our node
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Getting all IPRules from our node"))
	actualIPRulesOnNode, err = getActualIPRuleEntryMapFromNode(nh)
	if err != nil {
		log.Error(err, fmt.Sprintf("[NIRSReconciler] unable to get Actual IPRules from node"))
		return true, err
	}
	if len(actualIPRulesOnNode) == 0 {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] There are no IPRules with Realm=" + strconv.Itoa(v1alpha1.D8Realm)))
	}

	for _, nirs := range nirsList.Items {
		nirsSummary := nirsSummaryInit()
		// Gathering facts
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting gather facts about nirs %v", nirs.Name))
		if nirsSummary.discoverFacts(nirs, &globalDesiredIPRulesForNode, &actualIPRulesOnNode, log) {
			(*nirsMap)[nirs.Name] = nirsSummary
			continue
		}

		// Actions: add IPRules
		if len(nirsSummary.desiredIPRulesToAddByNIRS) > 0 {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting adding IPRules to the node"))
			nirsSummary.addIPRules(&actualIPRulesOnNode, nh, log)
		}

		(*nirsMap)[nirs.Name] = nirsSummary
	}

	// Actions: delete IPRules and finalizers
	nirsMap.deleteIPRulesAndFinalizers(globalDesiredIPRulesForNode, actualIPRulesOnNode, nh, log)

	// Actions: Deleting orphan IPRules (with realm 216) that are not mentioned in any NIRS
	deleteOrphanIPRules(globalDesiredIPRulesForNode, actualIPRulesOnNode, nh, log)

	// Generate new condition for each processed nirs
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting generate new conditions"))
	shouldRequeue := nirsMap.generateNewCondition()

	// Update state in k8s for each processed nirs
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting updating resourses in k8s"))
	nirsMap.updateStateInK8S(ctx, cl, log)

	return shouldRequeue, nil
}

func periodicalRunEventIPRuleReconcile(
	ctx context.Context,
	cfg config.Options,
	cl client.Client,
	nh netns.NsHandle,
	log logr.Logger,
	nodeName string,
) {
	ticker := time.NewTicker(cfg.PeriodicReconciliationInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starts a periodic reconciliation (initiated by a timer)"))
			_, err := runEventIPRuleReconcile(ctx, cl, nh, log, nodeName)
			if err != nil {
				log.Error(err, fmt.Sprintf("[NIRSReconciler] an error occurred while IPRule reconcile"))
			}
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Ends a periodic reconciliation (initiated by a timer)"))
		case <-ctx.Done():
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Completion of periodic reconciliations"))
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

func (ire *IPRuleEntry) String() string {
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

func (ire *IPRuleEntry) getHash() string {
	hash, err := hashstructure.Hash(*ire, hashstructure.FormatV2, nil)
	if err != nil {
		return ire.String()
	}

	return fmt.Sprintf("%v", hash)
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

			if ipRule.Selectors.SPortRange.Start != 0 {
				ire.SPortRange = &v1alpha1.PortRange{
					Start: ipRule.Selectors.SPortRange.Start,
				}
				if ipRule.Selectors.SPortRange.End != 0 {
					ire.SPortRange.End = ipRule.Selectors.SPortRange.End
				} else {
					ire.SPortRange.End = ipRule.Selectors.SPortRange.Start
				}
			}

			if ipRule.Selectors.DPortRange.Start != 0 {
				ire.DPortRange = &v1alpha1.PortRange{
					Start: ipRule.Selectors.DPortRange.Start,
				}
				if ipRule.Selectors.DPortRange.End != 0 {
					ire.DPortRange.End = ipRule.Selectors.DPortRange.End
				} else {
					ire.DPortRange.End = ipRule.Selectors.DPortRange.Start
				}
			}

			if ipRule.Selectors.UIDRange.Start != 0 {
				ire.UIDRange = &v1alpha1.UIDRange{
					Start: ipRule.Selectors.UIDRange.Start,
				}
				if ipRule.Selectors.UIDRange.End != 0 {
					ire.UIDRange.End = ipRule.Selectors.UIDRange.End
				} else {
					ire.UIDRange.End = ipRule.Selectors.UIDRange.Start
				}
			}

			ire.Src = from
			ire.Dst = to

			(*irem)[ire.getHash()] = ire
		}
	}
}

// nirsSummary: type, service functions and methods

type nirsSummary struct {
	k8sResources              *v1alpha1.SDNInternalNodeIPRuleSet
	newReconciliationStatus   utils.ReconciliationStatus
	desiredIPRulesByNIRS      IPRuleEntryMap
	lastAppliedIPRulesByNIRS  IPRuleEntryMap
	desiredIPRulesToAddByNIRS []IPRuleEntry
	desiredIPRulesToDelByNIRS IPRuleEntryMap
	nirsWasDeleted            bool
	needToWipeFinalizer       bool
}

func nirsSummaryInit() *nirsSummary {
	return &nirsSummary{
		k8sResources:              new(v1alpha1.SDNInternalNodeIPRuleSet),
		newReconciliationStatus:   utils.ReconciliationStatus{},
		desiredIPRulesByNIRS:      IPRuleEntryMap{},
		lastAppliedIPRulesByNIRS:  IPRuleEntryMap{},
		desiredIPRulesToAddByNIRS: make([]IPRuleEntry, 0),
		desiredIPRulesToDelByNIRS: IPRuleEntryMap{},
		nirsWasDeleted:            false,
		needToWipeFinalizer:       false,
	}
}

func (ns *nirsSummary) discoverFacts(nirs v1alpha1.SDNInternalNodeIPRuleSet, globalDesiredIPRulesForNode, actualIPRulesOnNode *IPRuleEntryMap, log logr.Logger) bool {
	// Filling nirsK8sResourcesMap[nirs.Name] and nirsReconciliationStatusMap[nirs.Name]
	tmpNIRS := nirs
	tmpNIRS.Status.ObservedGeneration = nirs.Generation
	ns.k8sResources = &tmpNIRS
	ns.newReconciliationStatus = utils.ReconciliationStatus{IsSuccess: true}
	ns.needToWipeFinalizer = false

	// If NIRS was deleted filling map desiredIPRulesToDelByNIRS and set flag nirsWasDeleted
	if nirs.DeletionTimestamp != nil {
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] NIRS %v is marked for deletion", nirs.Name))
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToDel, and set flag nirsWasDeleted "))
		tmpIREM := make(IPRuleEntryMap)
		for _, ipRule := range nirs.Spec.IPRules {
			tmpIREM.AppendIR(ipRule)
		}
		ns.desiredIPRulesToDelByNIRS = tmpIREM
		ns.nirsWasDeleted = true
		return true
	}

	// Filling desiredIPRulesByNRT and globalDesiredIPRulesForNode
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting filling maps: desiredIPRules and globalDesiredIPRules"))
	for _, ipRule := range nirs.Spec.IPRules {
		ns.desiredIPRulesByNIRS.AppendIR(ipRule)
		globalDesiredIPRulesForNode.AppendIR(ipRule)
	}

	// Filling lastAppliedRoutesByNRT
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting filling map lastAppliedIPRules"))
	if nirs.Status.AppliedIPRules != nil {
		for _, ipRule := range nirs.Status.AppliedIPRules {
			ns.lastAppliedIPRulesByNIRS.AppendIR(ipRule)
		}
	}

	// Filling desiredIPRulesToAddByNIRS
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToAdd"))
	for hash, desiredIPRule := range ns.desiredIPRulesByNIRS {
		if _, ok := (*actualIPRulesOnNode)[hash]; !ok {
			ns.desiredIPRulesToAddByNIRS = append(ns.desiredIPRulesToAddByNIRS, desiredIPRule)
		}
	}

	// Filling desiredIPRulesToDelByNIRS
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting filling map ipRulesToDel"))
	tmpIREM := make(IPRuleEntryMap)
	for hash, ipRule := range ns.lastAppliedIPRulesByNIRS {
		if _, ok := ns.desiredIPRulesByNIRS[hash]; !ok {
			tmpIREM.AppendIRE(ipRule)
		}
	}
	ns.desiredIPRulesToDelByNIRS = tmpIREM

	return false
}

func (ns *nirsSummary) addIPRules(actualIPRulesOnNode *IPRuleEntryMap, nh netns.NsHandle, log logr.Logger) {
	status := ns.newReconciliationStatus
	for _, ipRule := range ns.desiredIPRulesToAddByNIRS {
		log.V(config.DebugLvl).Info(fmt.Sprintf("IPRule %v should be added", ipRule))
		if _, ok := (*actualIPRulesOnNode)[ipRule.getHash()]; ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("but it is already present on Node"))
			continue
		}
		err := addIPRuleToNode(nh, ipRule)
		if err == nil {
			actualIPRulesOnNode.AppendIRE(ipRule)
		} else {
			log.V(config.DebugLvl).Info(fmt.Sprintf("err: %v", err))
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

func (nm *nirsMap) deleteIPRulesAndFinalizers(globalDesiredIPRulesForNode, actualIPRulesOnNode IPRuleEntryMap, nh netns.NsHandle, log logr.Logger) {
	for nirsName, ns := range *nm {
		if len(ns.desiredIPRulesToDelByNIRS) == 0 && !ns.nirsWasDeleted {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] NIRS %v has no entries in desiredIPRulesToDelByNIRS and DeletionTimestamp is not set", nirsName))
			continue
		}
		log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting to delete IPRules deleted from NIRS %v from node", nirsName))
		status := ns.newReconciliationStatus
		ns.newReconciliationStatus = deleteIPRuleEntriesFromNode(
			ns.desiredIPRulesToDelByNIRS,
			globalDesiredIPRulesForNode,
			&actualIPRulesOnNode,
			status,
			nh,
			log,
		)
		if ns.nirsWasDeleted && ns.newReconciliationStatus.IsSuccess {
			log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] NIRS %v has been deleted and its IPRules has been successfully deleted too. The finalizer will be wiped", nirsName))
			ns.needToWipeFinalizer = true
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

func (nm *nirsMap) updateStateInK8S(ctx context.Context, cl client.Client, log logr.Logger) {
	for nirsName, ns := range *nm {
		// Wipe the finalizer if necessary
		if ns.needToWipeFinalizer && ns.k8sResources.DeletionTimestamp != nil {
			log.V(config.DebugLvl).Info(fmt.Sprintf("Wipe finalizer on NIRS: %v", nirsName))

			tmpNIRSFinalizers := make([]string, 0)
			for _, fnlzr := range ns.k8sResources.Finalizers {
				if fnlzr != v1alpha1.Finalizer {
					tmpNIRSFinalizers = append(tmpNIRSFinalizers, fnlzr)
				}
			}

			patch, err := json.Marshal(
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": tmpNIRSFinalizers,
					},
				},
			)
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to marshal patch for finalizers %v, err: %v", tmpNIRSFinalizers, err))
			}

			err = cl.Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to patch CR SDNInternalNodeIPRuleSet %v, err: %v", nirsName, err))
			}
		}

		// Update status every time
		log.V(config.DebugLvl).Info(fmt.Sprintf("Update status of NIRS: %v", nirsName))

		patch, err := json.Marshal(
			map[string]interface{}{
				"status": ns.k8sResources.Status,
			},
		)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to marshal patch for status %v, err: %v", ns.k8sResources.Status, err))
		}

		err = cl.Status().Patch(ctx, ns.k8sResources, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to patch status for CR SDNInternalNodeIPRuleSet %v, err: %v", nirsName, err))
		}
	}
}

// netlink service functions

func addIPRuleToNode(nsHandle netns.NsHandle, ipRule IPRuleEntry) error {
	PreparedIPRule, err := ipRule.getNetlinkRule()
	if err != nil {
		return fmt.Errorf("unable to parse IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	PreparedIPRule.Flow = v1alpha1.D8Realm

	nh, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return fmt.Errorf("failed create new netlink handler, err: %w", err)
	}
	defer nh.Close()

	err = nh.RuleAdd(PreparedIPRule)
	if err != nil {
		return fmt.Errorf("unable to add IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	return nil
}

func delIPRuleFromNode(nsHandle netns.NsHandle, ipRule IPRuleEntry) error {
	PreparedIPRule, err := ipRule.getNetlinkRule()
	if err != nil {
		return fmt.Errorf("unable to parse IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	PreparedIPRule.Flow = v1alpha1.D8Realm

	nh, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return fmt.Errorf("failed create new netlink handler, err: %w", err)
	}
	defer nh.Close()

	err = nh.RuleDel(PreparedIPRule)
	if err != nil {
		return fmt.Errorf("unable to del IPRule %v, err: %w",
			ipRule,
			err,
		)
	}
	return nil
}

func getActualIPRuleEntryMapFromNode(nsHandle netns.NsHandle) (IPRuleEntryMap, error) {
	nh, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed create new netlink handler, err: %w", err)
	}
	defer nh.Close()

	nlRules, err := nh.RuleListFiltered(netlink.FAMILY_V4, nil, 0)
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

func deleteIPRuleEntriesFromNode(delIREM, gdIREM IPRuleEntryMap, actIREM *IPRuleEntryMap, status utils.ReconciliationStatus, nh netns.NsHandle, log logr.Logger) utils.ReconciliationStatus {
	for hash, ipRule := range delIREM {
		log.V(config.DebugLvl).Info(fmt.Sprintf("IPRule %v should be deleted", ipRule))
		if _, ok := (gdIREM)[hash]; ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("but it is present in other NIRS"))
			continue
		}
		if _, ok := (*actIREM)[hash]; !ok {
			log.V(config.DebugLvl).Info(fmt.Sprintf("but it is not present on Node"))
			continue
		}
		err := delIPRuleFromNode(nh, ipRule)
		if err == nil {
			delete(*actIREM, hash)
		} else {
			log.V(config.DebugLvl).Info(fmt.Sprintf("err: %v", err))
			status.AppendError(err)
		}
	}
	return status
}

func deleteOrphanIPRules(gdIREM, actIREM IPRuleEntryMap, nh netns.NsHandle, log logr.Logger) {
	log.V(config.DebugLvl).Info(fmt.Sprintf("[NIRSReconciler] Starting to find and delete orphan IPRules (with realm %v) from node.", v1alpha1.D8Realm))
	for hash, ipRule := range actIREM {
		if _, ok := (gdIREM)[hash]; ok {
			continue
		}
		log.V(config.DebugLvl).Info(fmt.Sprintf("ipRule %v should be deleted.", ipRule))
		err := delIPRuleFromNode(nh, ipRule)
		if err != nil {
			log.V(config.DebugLvl).Info(fmt.Sprintf("Unable to delete ipRule %v,err: %v", ipRule, err))
		}
	}
}
