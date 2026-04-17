/*
Copyright 2026 Flant JSC

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

package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionscalc "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditionscalc"
)

func ConvertToCalcConditions(conds []metav1.Condition) []conditionscalc.NodeGroupCondition {
	result := make([]conditionscalc.NodeGroupCondition, 0, len(conds))
	for _, c := range conds {
		ngCond := conditionscalc.NodeGroupCondition{
			Type:               conditionscalc.NodeGroupConditionType(c.Type),
			LastTransitionTime: c.LastTransitionTime,
		}
		switch c.Status {
		case metav1.ConditionTrue:
			ngCond.Status = conditionscalc.ConditionTrue
		case metav1.ConditionFalse:
			ngCond.Status = conditionscalc.ConditionFalse
		default:
			ngCond.Status = conditionscalc.ConditionFalse
		}
		ngCond.Message = c.Message
		result = append(result, ngCond)
	}
	return result
}

func ConvertFromCalcConditions(conds []conditionscalc.NodeGroupCondition) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(conds))
	for _, c := range conds {
		cond := metav1.Condition{
			Type:               string(c.Type),
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		}
		switch c.Status {
		case conditionscalc.ConditionTrue:
			cond.Status = metav1.ConditionTrue
		case conditionscalc.ConditionFalse:
			cond.Status = metav1.ConditionFalse
		default:
			cond.Status = metav1.ConditionFalse
		}
		result = append(result, cond)
	}
	return result
}
