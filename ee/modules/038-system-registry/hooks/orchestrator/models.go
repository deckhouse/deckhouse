/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type Params struct {
	Generation int64
	Mode       registry_const.ModeType
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
}

type Inputs struct {
	Params Params

	PKI          pki.Inputs
	Secrets      secrets.Inputs
	Users        users.Inputs
	NodeServices nodeservices.Inputs
}

type State struct {
	Mode registry_const.ModeType `json:"mode,omitempty"`

	PKI          *pki.State          `json:"pki,omitempty"`
	Secrets      *secrets.State      `json:"secrets,omitempty"`
	Users        *users.State        `json:"users,omitempty"`
	NodeServices *nodeservices.State `json:"node_services,omitempty"`
}

type Values struct {
	ProcessResult

	Hash  string `json:"hash,omitempty"`
	State State  `json:"state,omitempty"`
}

type ProcessResult struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (result *ProcessResult) SetCondition(condition metav1.Condition) {
	existingCondition := result.FindCondition(condition.Type)

	if existingCondition == nil {
		// Condition doesn't exist, add it
		condition.LastTransitionTime = metav1.NewTime(time.Now())
		result.Conditions = append(result.Conditions, condition)
		return
	}

	// Only update if something changed
	if existingCondition.Status != condition.Status ||
		existingCondition.Reason != condition.Reason ||
		existingCondition.Message != condition.Message {
		// Status changed, update transition time
		if existingCondition.Status != condition.Status {
			condition.LastTransitionTime = metav1.NewTime(time.Now())
		} else {
			condition.LastTransitionTime = existingCondition.LastTransitionTime
		}

		// Replace the existing condition
		*existingCondition = condition
	}
}

func (result *ProcessResult) FindCondition(conditionType string) *metav1.Condition {
	for i := range result.Conditions {
		if result.Conditions[i].Type == conditionType {
			return &result.Conditions[i]
		}
	}
	return nil
}

func (result *ProcessResult) RemoveCondition(conditionType string) {
	newConditions := make([]metav1.Condition, 0, len(result.Conditions))
	for _, c := range result.Conditions {
		if c.Type != conditionType {
			newConditions = append(newConditions, c)
		}
	}
	result.Conditions = newConditions
}

func (result *ProcessResult) IsConditionTrue(conditionType string) bool {
	condition := result.FindCondition(conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func (result *ProcessResult) ClearConditions() {
	result.Conditions = nil
}
