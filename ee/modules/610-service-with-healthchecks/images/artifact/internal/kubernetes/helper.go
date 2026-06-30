/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SortConditions(conditions []metav1.Condition) {
	sort.Slice(conditions, func(i, j int) bool {
		return conditions[i].Type < conditions[j].Type
	})
}

func UpdateStatusWithCondition(existingConditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, existingCondition := range existingConditions {
		if existingCondition.Type == newCondition.Type {
			if existingCondition.Status == newCondition.Status &&
				existingCondition.Reason == newCondition.Reason &&
				existingCondition.Message == newCondition.Message &&
				existingCondition.ObservedGeneration == newCondition.ObservedGeneration {
				return existingConditions
			}
			existingConditions[i] = newCondition
			return existingConditions
		}
	}
	return append(existingConditions, newCondition)
}

func UpdateStatusWithConditions(existingConditions []metav1.Condition, newConditions []metav1.Condition) []metav1.Condition {
	resultConditions := existingConditions
	for _, newCondition := range newConditions {
		resultConditions = UpdateStatusWithCondition(resultConditions, newCondition)
	}
	return resultConditions
}
