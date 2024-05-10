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

package updater

import (
	"time"

	"github.com/Masterminds/semver/v3"
)

type ByVersion[R Release] []R

func (a ByVersion[R]) Len() int {
	return len(a)
}
func (a ByVersion[R]) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a ByVersion[R]) Less(i, j int) bool {
	return a[i].GetVersion().LessThan(a[j].GetVersion())
}

type DeckhouseReleaseData struct {
	IsUpdating bool
	Notified   bool
}

type Release interface {
	GetName() string
	GetApplyAfter() *time.Time
	GetVersion() *semver.Version
	GetRequirements() map[string]string
	GetChangelogLink() string
	GetCooldownUntil() *time.Time
	GetDisruptions() []string
	GetDisruptionApproved() bool
	GetPhase() string
	GetForce() bool
	GetApplyNow() bool
	GetApprovedStatus() bool
	SetApprovedStatus(b bool)
	GetSuspend() bool
	GetManuallyApproved() bool
	GetMessage() string
}

type KubeAPI[R Release] interface {
	UpdateReleaseStatus(release R, msg, phase string) error
	PatchReleaseAnnotations(release R, annotations map[string]interface{}) error
	PatchReleaseApplyAfter(release R, applyTime time.Time) error
	SaveReleaseData(release R, data DeckhouseReleaseData) error
	DeployRelease(release R) error
}
