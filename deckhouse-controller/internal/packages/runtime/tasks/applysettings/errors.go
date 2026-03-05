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

package applysettings

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Condition reasons for settings-related failures.
const (
	ConditionReasonValidationFailed status.ConditionReason = "ValidationFailed"
	ConditionReasonApplySettings    status.ConditionReason = "ApplySettings"
)

// newApplySettingsErr wraps an error with conditions that mark both
// ReadyInRuntime and SettingsValid as False. This ensures the status
// service properly reflects validation or apply failures.
func newApplySettingsErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyInRuntime,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonApplySettings,
				Message: err.Error(),
			},
			{
				Type:    status.ConditionSettingsValid,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonValidationFailed,
				Message: err.Error(),
			},
		},
	}
}
