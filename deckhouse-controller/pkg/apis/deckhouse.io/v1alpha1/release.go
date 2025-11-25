/*
Copyright 2024 Flant JSC

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

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
)

// +kubebuilder:object:generate=false
type Release interface {
	GetName() string
	GetModuleName() string
	GetApplyAfter() *time.Time
	GetVersion() *semver.Version
	GetRequirements() map[string]string
	GetChangelogLink() string
	GetDisruptions() []string
	GetDisruptionApproved() bool
	GetPhase() string
	GetForce() bool
	GetReinstall() bool
	GetApplyNow() bool
	GetApprovedStatus() bool
	SetApprovedStatus(b bool)
	GetSuspend() bool
	GetManuallyApproved() bool
	GetMessage() string
	GetNotified() bool
	GetUpdateSpec() *UpdateSpec
}

// +kubebuilder:pruning:PreserveUnknownFields
type Changelog map[string]any

func GetReleaseApprovalAnnotation(release Release) string {
	switch release.(type) {
	case *DeckhouseRelease:
		return DeckhouseReleaseApprovalAnnotation
	case *ModuleRelease:
		return ModuleReleaseApprovalAnnotation
	default:
		panic(fmt.Sprintf("cannot find approval annotation: unsupported release type %T", release))
	}
}
