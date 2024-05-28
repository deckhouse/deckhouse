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

package utils

import (
	"context"
	"fmt"
	"static-routing-manager-agent/api/v1alpha1"
	"static-routing-manager-agent/pkg/logger"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
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

func SetStatusConditionPendingToNIRS(ctx context.Context, cl client.Client, log logger.Logger, nirs *v1alpha1.NodeIPRuleSet) error {
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

func SetStatusConditionPendingToNRT(ctx context.Context, cl client.Client, log logger.Logger, nrt *v1alpha1.NodeRoutingTable) error {
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
		log.Error(err, fmt.Sprintf("unable to update status for CR NodeRoutingTable %v, err: %v", nrt.Name, err))
		return err
	}
	return nil
}
