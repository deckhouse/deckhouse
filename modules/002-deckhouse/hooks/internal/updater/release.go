/*
Copyright 2022 Flant JSC

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

package d8updater

import (
	"encoding/json"
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type DeckhouseRelease struct {
	Name    string
	Version *semver.Version

	ManuallyApproved bool
	AnnotationFlags  DeckhouseReleaseAnnotationsFlags

	Requirements  map[string]string
	ChangelogLink string
	Disruptions   []string
	ApplyAfter    *metav1.Time
	CooldownUntil *metav1.Time

	Status v1alpha1.DeckhouseReleaseStatus // don't set transition time here to avoid snapshot overload
}

func (d *DeckhouseRelease) GetName() string {
	return d.Name
}

func (d *DeckhouseRelease) GetApplyAfter() *time.Time {
	if d.ApplyAfter == nil {
		return nil
	}
	return &d.ApplyAfter.Time
}

func (d *DeckhouseRelease) GetVersion() *semver.Version {
	return d.Version
}

func (d *DeckhouseRelease) GetRequirements() map[string]string {
	return d.Requirements
}

func (d *DeckhouseRelease) GetChangelogLink() string {
	return d.ChangelogLink
}

func (d *DeckhouseRelease) GetCooldownUntil() *time.Time {
	if d.CooldownUntil == nil {
		return nil
	}
	return &d.CooldownUntil.Time
}

func (d *DeckhouseRelease) GetDisruptions() []string {
	return d.Disruptions
}

func (d *DeckhouseRelease) GetDisruptionApproved() bool {
	return d.AnnotationFlags.DisruptionApproved
}

func (d *DeckhouseRelease) GetPhase() string {
	return d.Status.Phase
}

func (d *DeckhouseRelease) GetForce() bool {
	return d.AnnotationFlags.Force
}

func (d *DeckhouseRelease) GetApplyNow() bool {
	return d.AnnotationFlags.ApplyNow
}

func (d *DeckhouseRelease) GetApprovedStatus() bool {
	return d.Status.Approved
}

func (d *DeckhouseRelease) SetApprovedStatus(val bool) {
	d.Status.Approved = val
}

func (d *DeckhouseRelease) GetSuspend() bool {
	return d.AnnotationFlags.Suspend
}

func (d *DeckhouseRelease) GetManuallyApproved() bool {
	return d.ManuallyApproved
}

func (d *DeckhouseRelease) GetMessage() string {
	return d.Status.Message
}

type DeckhouseReleaseAnnotationsFlags struct {
	Suspend            bool
	Force              bool
	ApplyNow           bool
	DisruptionApproved bool
	NotificationShift  bool // time shift by the notification process
}

type StatusPatch v1alpha1.DeckhouseReleaseStatus

func (sp StatusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.DeckhouseReleaseStatus(sp),
	}

	return json.Marshal(m)
}
