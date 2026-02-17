// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package run

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Condition reasons for run-related failures.
const (
	ConditionReasonBeforeHelmHooksFailed status.ConditionReason = "BeforeHelmHooksFailed"
	ConditionReasonAfterHelmHooksFailed  status.ConditionReason = "AfterHelmHooksFailed"
	ConditionReasonHooksFailed           status.ConditionReason = "HooksFailed"
	ConditionReasonHelmUpgradeFailed     status.ConditionReason = "HelmUpgradeFailed"
)

// newHelmUpgradeErr wraps an error when Helm install/upgrade fails.
// Sets ReadyInRuntime and ReadyInCluster to False.
func newHelmUpgradeErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyInRuntime,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonHelmUpgradeFailed,
				Message: err.Error(),
			},
			{
				Type:    status.ConditionReadyInCluster,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonHelmUpgradeFailed,
				Message: err.Error(),
			},
		},
	}
}

// newBeforeHelmHookErr wraps an error when BeforeHelm hooks fail.
// Sets HooksProcessed and ReadyInRuntime to False.
func newBeforeHelmHookErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionHooksProcessed,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonBeforeHelmHooksFailed,
				Message: err.Error(),
			},
			{
				Type:    status.ConditionReadyInRuntime,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonHooksFailed,
				Message: err.Error(),
			},
		},
	}
}

// newAfterHelmHookErr wraps an error when AfterHelm hooks fail.
// Sets HooksProcessed and ReadyInRuntime to False.
func newAfterHelmHookErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionHooksProcessed,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonAfterHelmHooksFailed,
				Message: err.Error(),
			},
			{
				Type:    status.ConditionReadyInRuntime,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonHooksFailed,
				Message: err.Error(),
			},
		},
	}
}
