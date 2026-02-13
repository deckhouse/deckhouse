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

func newDownloadErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionDownloaded,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonDownload,
				Message: err.Error(),
			},
		},
	}
}

func newCreatePackageDirErr(err error) error {
	return &status.Error{
		Err: err,
		Conditions: []status.Condition{
			{
				Type:    status.ConditionDownloaded,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonCreatePackageDir,
				Message: err.Error(),
			},
		},
	}
}

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
