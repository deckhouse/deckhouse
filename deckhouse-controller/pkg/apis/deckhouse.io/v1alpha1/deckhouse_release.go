/*
Copyright 2021 Flant JSC

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
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DeckhouseReleasePhasePending    = "Pending"
	DeckhouseReleasePhaseDeployed   = "Deployed"
	DeckhouseReleasePhaseSuperseded = "Superseded"
	DeckhouseReleasePhaseSuspended  = "Suspended"
	DeckhouseReleasePhaseSkipped    = "Skipped"

	DeckhouseReleaseApprovalAnnotation              = "release.deckhouse.io/approved"
	DeckhouseReleaseAnnotationIsUpdating            = "release.deckhouse.io/isUpdating"
	DeckhouseReleaseAnnotationNotified              = "release.deckhouse.io/notified"
	DeckhouseReleaseAnnotationApplyNow              = "release.deckhouse.io/apply-now"
	DeckhouseReleaseAnnotationApplyAfter            = "release.deckhouse.io/applyAfter"
	DeckhouseReleaseAnnotationDisruptionApproved    = "release.deckhouse.io/disruption-approved"
	DeckhouseReleaseAnnotationForce                 = "release.deckhouse.io/force"
	DeckhouseReleaseAnnotationSuspended             = "release.deckhouse.io/suspended"
	DeckhouseReleaseAnnotationNotificationTimeShift = "release.deckhouse.io/notification-time-shift"
	DeckhouseReleaseAnnotationCurrentRestored       = "release.deckhouse.io/current-restored"
	DeckhouseReleaseAnnotationChangeCause           = "release.deckhouse.io/change-cause"
	DeckhouseReleaseAnnotationUpdateInfo            = "release.deckhouse.io/update-info"

	DeckhouseReleaseAnnotationDryrun            = "dryrun"
	DeckhouseReleaseAnnotationTriggeredByDryrun = "triggered_by_dryrun"
)

var DeckhouseReleaseGVK = schema.GroupVersionKind{
	Group:   SchemeGroupVersion.Group,
	Version: SchemeGroupVersion.Version,
	Kind:    DeckhouseReleaseKind,
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// DeckhouseRelease is a deckhouse release object.
type DeckhouseRelease struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Approved bool `json:"approved"`

	Spec DeckhouseReleaseSpec `json:"spec"`

	Status DeckhouseReleaseStatus `json:"status,omitempty"`
}

func (in *DeckhouseRelease) GetApplyAfter() *time.Time {
	if in.Spec.ApplyAfter == nil {
		return nil
	}
	return &in.Spec.ApplyAfter.Time
}

func (in *DeckhouseRelease) GetVersion() *semver.Version {
	return semver.MustParse(in.Spec.Version)
}

func (in *DeckhouseRelease) GetRequirements() map[string]string {
	return in.Spec.Requirements
}

func (in *DeckhouseRelease) GetChangelogLink() string {
	return in.Spec.ChangelogLink
}

func (in *DeckhouseRelease) GetDisruptions() []string {
	return in.Spec.Disruptions
}

func (in *DeckhouseRelease) GetDisruptionApproved() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationDisruptionApproved]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetPhase() string {
	return in.Status.Phase
}

func (in *DeckhouseRelease) GetForce() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationForce]
	return ok && v == "true"
}

func (*DeckhouseRelease) GetReinstall() bool {
	return false
}

func (in *DeckhouseRelease) GetApplyNow() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationApplyNow]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetIsUpdating() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationIsUpdating]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetNotified() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationNotified]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetApprovedStatus() bool {
	return in.Status.Approved
}

func (in *DeckhouseRelease) SetApprovedStatus(val bool) {
	in.Status.Approved = val
}

func (in *DeckhouseRelease) GetSuspend() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationSuspended]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetManuallyApproved() bool {
	if in.Approved {
		return true
	}

	v, ok := in.Annotations[DeckhouseReleaseApprovalAnnotation]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetMessage() string {
	return in.Status.Message
}

func (in *DeckhouseRelease) GetNotificationShift() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationNotificationTimeShift]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetDryRun() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationDryrun]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetTriggeredByDryRun() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationTriggeredByDryrun]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetCurrentRestored() bool {
	v, ok := in.Annotations[DeckhouseReleaseAnnotationCurrentRestored]
	return ok && v == "true"
}

func (in *DeckhouseRelease) GetModuleName() string {
	return ""
}

// GetUpdateSpec returns the optional update spec of the related release
func (in *DeckhouseRelease) GetUpdateSpec() *UpdateSpec {
	return nil
}

type DeckhouseReleaseSpec struct {
	Version      string            `json:"version,omitempty"`
	ApplyAfter   *metav1.Time      `json:"applyAfter,omitempty"`
	Requirements map[string]string `json:"requirements,omitempty"`
	Disruptions  []string          `json:"disruptions,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Changelog     Changelog `json:"changelog,omitempty"`
	ChangelogLink string    `json:"changelogLink,omitempty"`
}

type DeckhouseReleaseStatus struct {
	Phase          string      `json:"phase,omitempty"`
	Approved       bool        `json:"approved"`
	TransitionTime metav1.Time `json:"transitionTime,omitempty"`
	Message        string      `json:"message"`
}

type deckhouseReleaseKind struct{}

func (in *DeckhouseReleaseStatus) GetObjectKind() schema.ObjectKind {
	return &deckhouseReleaseKind{}
}

func (f *deckhouseReleaseKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *deckhouseReleaseKind) GroupVersionKind() schema.GroupVersionKind {
	return DeckhouseReleaseGVK
}

// +kubebuilder:object:root=true

// DeckhouseReleaseList is a list of DeckhouseRelease resources
type DeckhouseReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []DeckhouseRelease `json:"items"`
}
