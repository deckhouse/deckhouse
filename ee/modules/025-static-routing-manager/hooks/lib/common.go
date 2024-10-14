/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package lib

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/hooks/lib/v1alpha1"
)

type NodeInfo struct {
	Name   string
	Labels map[string]string
}

func ApplyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

func GenerateShortHash(input string) string {
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	if len(fullHash) > 10 {
		return fullHash[:10]
	}
	return fullHash
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

func DeleteFinalizer(input *go_hook.HookInput, crName, crAPIVersion, crKind, finalizerToMatch string) {
	input.PatchCollector.Filter(
		func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			objCopy := obj.DeepCopy()
			crFinalizers := objCopy.GetFinalizers()
			tmpFinalizers := make([]string, 0)
			for _, fnlzr := range crFinalizers {
				if fnlzr != finalizerToMatch {
					tmpFinalizers = append(tmpFinalizers, fnlzr)
				}
			}
			objCopy.SetFinalizers(tmpFinalizers)
			return objCopy, nil
		},
		crAPIVersion,
		crKind,
		"",
		crName,
	)
}
