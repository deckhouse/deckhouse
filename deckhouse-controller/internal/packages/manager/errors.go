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

package manager

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/status"
)

const (
	// HooksProcessed reasons
	ConditionReasonStartupHooksFailed    status.ConditionReason = "StartupHookFailed"
	ConditionReasonBeforeHelmHooksFailed status.ConditionReason = "BeforeHelmHooksFailed"
	ConditionReasonAfterHelmHooksFailed  status.ConditionReason = "AfterHelmHooksFailed"

	// ReadyInRuntime reasons
	ConditionReasonHooksFailed       status.ConditionReason = "HooksFailed"
	ConditionReasonLoadFailed        status.ConditionReason = "LoadFailed"
	ConditionReasonApplySettings     status.ConditionReason = "ApplySettings"
	ConditionReasonHelmUpgradeFailed status.ConditionReason = "HelmUpgradeFailed"
	ConditionReasonInitHooksFailed   status.ConditionReason = "InitHooksFailed"
)

func newApplySettingsErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonApplySettings,
				Message: err.Error(),
			},
		},
	}
}

func newHelmUpgradeErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonHelmUpgradeFailed,
				Message: err.Error(),
			},
		},
	}
}

func newInitHooksErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonInitHooksFailed,
				Message: err.Error(),
			},
		},
	}
}

func newLoadFailedErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonLoadFailed,
				Message: err.Error(),
			},
		},
	}
}

func newStartupHookErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHooksProcessed,
				Status:  false,
				Reason:  ConditionReasonStartupHooksFailed,
				Message: err.Error(),
			},
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonHooksFailed,
				Message: err.Error(),
			},
		},
	}
}

func newBeforeHelmHookErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHooksProcessed,
				Status:  false,
				Reason:  ConditionReasonBeforeHelmHooksFailed,
				Message: err.Error(),
			},
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonHooksFailed,
				Message: err.Error(),
			},
		},
	}
}

func newAfterHelmHookErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHooksProcessed,
				Status:  false,
				Reason:  ConditionReasonAfterHelmHooksFailed,
				Message: err.Error(),
			},
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  false,
				Reason:  ConditionReasonHooksFailed,
				Message: err.Error(),
			},
		},
	}
}
