package nelm

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/status"
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
				Status:  false,
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
				Status:  false,
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
				Status:  false,
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
				Status:  false,
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
				Status:  false,
				Reason:  ConditionReasonInstallChart,
				Message: err.Error(),
			},
		},
	}
}
