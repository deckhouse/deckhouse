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
