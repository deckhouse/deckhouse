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

package installer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status:  metav1.ConditionFalse,
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
				Status: metav1.ConditionFalse,
				Reason: ConditionReasonMount,
			},
		},
	}
}
