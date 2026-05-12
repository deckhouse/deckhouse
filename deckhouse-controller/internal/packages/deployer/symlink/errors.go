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

package symlink

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

const (
	ConditionReasonDownload         status.ConditionReason = "Download"
	ConditionReasonCreatePackageDir status.ConditionReason = "CreatePackageDir"
	ConditionReasonRemoveOldVersion status.ConditionReason = "RemoveOldVersion"
	ConditionReasonCreateSymlink    status.ConditionReason = "CreateSymlink"
	ConditionReasonCheckMount       status.ConditionReason = "CheckMount"
	ConditionReasonCheckVersion     status.ConditionReason = "CheckVersion"
)

// newDownloadErr wraps err when the package image cannot be downloaded from the registry.
func newDownloadErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonDownload,
				Message: err.Error(),
			},
		},
	}
}

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

// newRemoveOldVersionErr wraps err when the previously installed version directory or symlink cannot be removed.
func newRemoveOldVersionErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonRemoveOldVersion,
				Message: err.Error(),
			},
		},
	}
}

// newCreateSymlinkErr wraps err when the versioned symlink pointing to the package directory cannot be created.
func newCreateSymlinkErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCreateSymlink,
				Message: err.Error(),
			},
		},
	}
}

// newCheckMountErr wraps err when the symlink target cannot be verified as a live mount.
func newCheckMountErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCheckMount,
				Message: err.Error(),
			},
		},
	}
}

// newCheckVersionErr wraps err when the installed package version does not match the expected version.
func newCheckVersionErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionReadyOnFilesystem,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCheckVersion,
				Message: err.Error(),
			},
		},
	}
}
