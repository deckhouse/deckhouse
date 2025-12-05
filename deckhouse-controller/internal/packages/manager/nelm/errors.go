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

package nelm

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

const (
	ConditionReasonRender           status.ConditionReason = "RenderFailed"
	ConditionReasonCheckChart       status.ConditionReason = "CheckChart"
	ConditionReasonCreateValuesFile status.ConditionReason = "CreateValuesFile"
	ConditionReasonRuntimeValues    status.ConditionReason = "MarshalRuntimeValues"
	ConditionReasonCheckRelease     status.ConditionReason = "CheckRelease"
	ConditionReasonInstallChart     status.ConditionReason = "InstallChart"
)

func newRenderError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonRender,
				Message: err.Error(),
			},
		},
	}
}

func newCheckChartError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCheckChart,
				Message: err.Error(),
			},
		},
	}
}

func newCreateValuesError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCreateValuesFile,
				Message: err.Error(),
			},
		},
	}
}

func newMarshalRuntimeValuesError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonRuntimeValues,
				Message: err.Error(),
			},
		},
	}
}

func newCheckReleaseError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCheckRelease,
				Message: err.Error(),
			},
		},
	}
}

func newInstallChartError(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionHelmApplied,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonInstallChart,
				Message: err.Error(),
			},
		},
	}
}
