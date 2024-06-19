/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/images/agent/api/v1alpha1"
)

type ReconciliationStatus struct {
	IsSuccess    bool
	ErrorMessage string
}

func (s *ReconciliationStatus) AppendError(err error) {
	s.IsSuccess = false
	if s.ErrorMessage == "" {
		s.ErrorMessage = err.Error()
	} else {
		s.ErrorMessage = s.ErrorMessage + "\n" + err.Error()
	}
}

func SetStatusCondition(conditions *[]v1alpha1.ExtendedCondition, newCondition v1alpha1.ExtendedCondition) (changed bool) {
	if conditions == nil {
		return false
	}

	timeNow := metav1.NewTime(time.Now())

	existingCondition := FindStatusCondition(*conditions, newCondition.Type)
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

func FindStatusCondition(conditions []v1alpha1.ExtendedCondition, conditionType string) *v1alpha1.ExtendedCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

//

func SetStatusConditionPendingToNIRS(ctx context.Context, cl client.Client, log logr.Logger, nirs *v1alpha1.SDNInternalNodeIPRuleSet) error {
	t := metav1.NewTime(time.Now())
	nirs.Status.ObservedGeneration = nirs.Generation

	newCond := v1alpha1.ExtendedCondition{}
	newCond.Type = v1alpha1.ReconciliationSucceedType
	newCond.LastHeartbeatTime = t
	newCond.Status = metav1.ConditionFalse
	newCond.Reason = v1alpha1.ReconciliationReasonPending
	newCond.Message = ""

	_ = SetStatusCondition(&nirs.Status.Conditions, newCond)

	err := cl.Status().Update(ctx, nirs)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to update status for CR NodeIPRuleSet %v, err: %v", nirs.Name, err))
		return err
	}
	return nil
}

func SetStatusConditionPendingToNRT(ctx context.Context, cl client.Client, log logr.Logger, nrt *v1alpha1.SDNInternalNodeRoutingTable) error {
	t := metav1.NewTime(time.Now())
	nrt.Status.ObservedGeneration = nrt.Generation

	newCond := v1alpha1.ExtendedCondition{}
	newCond.Type = v1alpha1.ReconciliationSucceedType
	newCond.LastHeartbeatTime = t
	newCond.Status = metav1.ConditionFalse
	newCond.Reason = v1alpha1.ReconciliationReasonPending
	newCond.Message = ""

	_ = SetStatusCondition(&nrt.Status.Conditions, newCond)

	err := cl.Status().Update(ctx, nrt)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to update status for CR SDNInternalNodeRoutingTable %v, err: %v", nrt.Name, err))
		return err
	}
	return nil
}

// netlink

func GetNetlinkNSHandlerByPath(pathToNS string) (*netlink.Handle, error) {

	if pathToNS == "" {
		pathToNS = "/hostproc/1/ns/net"
	}

	NsHandle, err := netns.GetFromPath(pathToNS)
	if err != nil {
		return nil, fmt.Errorf("failed get host namespace, err: %w", err)
	}
	nh, err := netlink.NewHandleAt(NsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed create new namespace handler, err: %w", err)
	}
	return nh, nil
}
