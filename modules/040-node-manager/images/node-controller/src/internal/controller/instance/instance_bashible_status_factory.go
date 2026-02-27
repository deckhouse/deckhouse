/*
Copyright 2025 Flant JSC

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

package instance

import (
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BashibleStatusFactory interface {
	FromConditions(conditions []deckhousev1alpha2.InstanceCondition) deckhousev1alpha2.BashibleStatus
}

type bashibleStatusFactory struct{}

func NewBashibleStatusFactory() BashibleStatusFactory {
	return &bashibleStatusFactory{}
}

func (f *bashibleStatusFactory) FromConditions(
	conditions []deckhousev1alpha2.InstanceCondition,
) deckhousev1alpha2.BashibleStatus {
	var bashibleReady *deckhousev1alpha2.InstanceCondition
	var waitingApproval *deckhousev1alpha2.InstanceCondition
	var waitingDisruptionApproval *deckhousev1alpha2.InstanceCondition

	for i := range conditions {
		condition := &conditions[i]
		switch condition.Type {
		case deckhousev1alpha2.InstanceConditionTypeBashibleReady:
			bashibleReady = condition
		case deckhousev1alpha2.InstanceConditionTypeWaitingApproval:
			waitingApproval = condition
		case deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval:
			waitingDisruptionApproval = condition
		}
	}

	if isConditionTrue(waitingDisruptionApproval) {
		return deckhousev1alpha2.BashibleStatusWaitingApproval
	}

	if bashibleReady == nil {
		return deckhousev1alpha2.BashibleStatusUnknown
	}

	switch bashibleReady.Status {
	case metav1.ConditionTrue:
		return deckhousev1alpha2.BashibleStatusReady
	case metav1.ConditionFalse:
		return deckhousev1alpha2.BashibleStatusError
	default:
		return deckhousev1alpha2.BashibleStatusUnknown
	}
}

func isConditionTrue(condition *deckhousev1alpha2.InstanceCondition) bool {
	return condition != nil && condition.Status == metav1.ConditionTrue
}
