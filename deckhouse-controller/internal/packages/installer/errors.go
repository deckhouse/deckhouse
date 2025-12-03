package installer

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/status"
)

const (
	ConditionReasonCreatePackageDir status.ConditionReason = "CreatePackagerDir"
	ConditionReasonGetRootHash      status.ConditionReason = "GetRootHash"
	ConditionReasonGetImageReader   status.ConditionReason = "GetImageReader"
	ConditionReasonImageByTar       status.ConditionReason = "ImageByTar"

	ConditionReasonUnmount            status.ConditionReason = "Unmount"
	ConditionReasonCloseDeviceMapper  status.ConditionReason = "CloseDeviceMapper"
	ConditionReasonComputeHash        status.ConditionReason = "ComputeHash"
	ConditionReasonCreateDeviceMapper status.ConditionReason = "CreateDeviceMapper"
	ConditionReasonMount              status.ConditionReason = "Mount"
)

func newCreatePackageDirErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionDownloaded,
				Status:  false,
				Reason:  ConditionReasonCreatePackageDir,
				Message: err.Error(),
			},
		},
	}
}

func newGetRootHashErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionDownloaded,
				Status:  false,
				Reason:  ConditionReasonGetRootHash,
				Message: err.Error(),
			},
		},
	}
}

func newGetImageReaderErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionDownloaded,
				Status:  false,
				Reason:  ConditionReasonGetImageReader,
				Message: err.Error(),
			},
		},
	}
}

func newImageByTarErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionDownloaded,
				Status:  false,
				Reason:  ConditionReasonImageByTar,
				Message: err.Error(),
			},
		},
	}
}

func newUnmountErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyOnFilesystem,
				Status:  false,
				Reason:  ConditionReasonUnmount,
				Message: err.Error(),
			},
		},
	}
}

func newCloseDeviceMapperErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyOnFilesystem,
				Status:  false,
				Reason:  ConditionReasonCloseDeviceMapper,
				Message: err.Error(),
			},
		},
	}
}

func newComputeHashErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyOnFilesystem,
				Status:  false,
				Reason:  ConditionReasonComputeHash,
				Message: err.Error(),
			},
		},
	}
}

func newCreateDeviceMapperErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyOnFilesystem,
				Status:  false,
				Reason:  ConditionReasonCreateDeviceMapper,
				Message: err.Error(),
			},
		},
	}
}

func newMountErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Name:   status.ConditionReadyOnFilesystem,
				Status: false,
				Reason: ConditionReasonMount,
			},
		},
	}
}
