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

package instance

import (
	"strings"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

const (
	conditionReasonDisruptionApprovalNotRequired = "DisruptionApprovalNotRequired"
	emptyMessage                                 = ""
	machineMessagePrefix                         = "machine: "
	bashibleMessagePrefix                        = "bashible: "
)

type MessageFactory interface {
	FromConditions(conditions []deckhousev1alpha2.InstanceCondition) string
}

type messageFactory struct{}

type conditionMatcher func(deckhousev1alpha2.InstanceCondition) bool

var messagePriorityMatchers = []conditionMatcher{
	func(c deckhousev1alpha2.InstanceCondition) bool {
		return c.Type == deckhousev1alpha2.InstanceConditionTypeMachineReady
	},
	func(c deckhousev1alpha2.InstanceCondition) bool {
		return c.Type == deckhousev1alpha2.InstanceConditionTypeBashibleReady
	},
	func(c deckhousev1alpha2.InstanceCondition) bool {
		return c.Reason == conditionReasonDisruptionApprovalNotRequired
	},
	func(c deckhousev1alpha2.InstanceCondition) bool {
		return c.Type == deckhousev1alpha2.InstanceConditionTypeWaitingApproval
	},
}

func NewMessageFactory() MessageFactory {
	return &messageFactory{}
}

func (f *messageFactory) FromConditions(conditions []deckhousev1alpha2.InstanceCondition) string {
	if message, ok := findConditionMessage(conditions, messagePriorityMatchers[0]); ok {
		return machineMessagePrefix + message
	}
	for _, match := range messagePriorityMatchers[1:] {
		if message, ok := findConditionMessage(conditions, match); ok {
			return bashibleMessagePrefix + message
		}
	}

	return emptyMessage
}

func findCondition(
	conditions []deckhousev1alpha2.InstanceCondition,
	match conditionMatcher,
) (deckhousev1alpha2.InstanceCondition, bool) {
	for i := range conditions {
		if match(conditions[i]) {
			return conditions[i], true
		}
	}

	return deckhousev1alpha2.InstanceCondition{}, false
}

func findConditionMessage(
	conditions []deckhousev1alpha2.InstanceCondition,
	match conditionMatcher,
) (string, bool) {
	condition, ok := findCondition(conditions, match)
	if !ok {
		return emptyMessage, false
	}

	message := strings.TrimSpace(condition.Message)
	if message == emptyMessage {
		return emptyMessage, false
	}

	return message, true
}
