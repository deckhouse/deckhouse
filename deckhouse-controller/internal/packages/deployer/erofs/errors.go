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

package erofs

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

const (
	ConditionReasonCreatePackageDir status.ConditionReason = "CreatePackageDir"
	ConditionReasonGetRootHash      status.ConditionReason = "GetRootHash"
	ConditionReasonGetImageReader   status.ConditionReason = "GetImageReader"
	ConditionReasonImageByTar       status.ConditionReason = "ImageByTar"

	ConditionReasonUnmount            status.ConditionReason = "Unmount"
	ConditionReasonCloseDeviceMapper  status.ConditionReason = "CloseDeviceMapper"
	ConditionReasonComputeHash        status.ConditionReason = "ComputeHash"
	ConditionReasonCreateDeviceMapper status.ConditionReason = "CreateDeviceMapper"
	ConditionReasonMount              status.ConditionReason = "Mount"
)

// newCreatePackageDirErr wraps err when the package directory cannot be created on the host filesystem.
func newCreatePackageDirErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCreatePackageDir,
				Message: err.Error(),
			},
		},
	}
}

// newGetRootHashErr wraps err when the dm-verity root hash cannot be extracted from the erofs image.
func newGetRootHashErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonGetRootHash,
				Message: err.Error(),
			},
		},
	}
}

// newGetImageReaderErr wraps err when an OCI image reader cannot be obtained from the registry.
func newGetImageReaderErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonGetImageReader,
				Message: err.Error(),
			},
		},
	}
}

// newImageByTarErr wraps err when the OCI image layer cannot be converted to an erofs image via tar.
func newImageByTarErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonImageByTar,
				Message: err.Error(),
			},
		},
	}
}

// newUnmountErr wraps err when the erofs filesystem cannot be unmounted from the host.
func newUnmountErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonUnmount,
				Message: err.Error(),
			},
		},
	}
}

// newCloseDeviceMapperErr wraps err when the dm-verity device-mapper device cannot be closed or removed.
func newCloseDeviceMapperErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCloseDeviceMapper,
				Message: err.Error(),
			},
		},
	}
}

// newComputeHashErr wraps err when the content hash of the erofs image cannot be computed.
func newComputeHashErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonComputeHash,
				Message: err.Error(),
			},
		},
	}
}

// newCreateDeviceMapperErr wraps err when the dm-verity device-mapper device cannot be created for the erofs image.
func newCreateDeviceMapperErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCreateDeviceMapper,
				Message: err.Error(),
			},
		},
	}
}

// newMountErr wraps err when the erofs filesystem cannot be mounted via the device-mapper device.
func newMountErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonMount,
				Message: err.Error(),
			},
		},
	}
}
