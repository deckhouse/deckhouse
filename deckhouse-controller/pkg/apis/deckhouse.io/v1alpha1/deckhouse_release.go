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
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="Current release status.\n\nTypical values:\n\n- `Pending`: The release has been created but not applied yet (waiting for manual approval, update windows, canary release, or the minimal notification time).\n- `Deployed`: DKP has switched to the release version (module and component updates may continue asynchronously).\n- `Superseded`: The release is outdated and no longer used.\n- `Suspended`: The release has been canceled (typically before it was applied).\n- `Skipped`: The release was skipped because the requirements were not satisfied.\n\nFor more information about DKP and modules release statuses, refer to the [`deckhouse` module documentation](/modules/deckhouse/#deckhouse-releases-update)."
// +kubebuilder:printcolumn:name="transitionTime",type="date",format="date-time",JSONPath=".status.transitionTime",description="When the release status was changed."
// +kubebuilder:printcolumn:name="message",type="string",JSONPath=".status.message",description="Release status details."
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=deckhouse"
// +crd-enricher:crd:preserveUnknownFields=false
// +crd-enricher:crd:minimal=true
// +crd-enricher:crd:stripFormat=true

// Determines the state and parameters for applying a specific release (version) of the Deckhouse Kubernetes Platform (DKP) in the cluster.
//
// DeckhouseRelease objects are automatically created by DKP upon the discovery of new DKP versions on the selected [release channel](/products/kubernetes-platform/documentation/latest/reference/release-channels.html). Modifying DeckhouseRelease enables the management of the process for applying the corresponding DKP version. More details about configuring DKP updates can be found in the [documentation](../../admin/configuration/update/configuration.html).
//
// The current versions of DKP and modules by release channels can be found at [releases.deckhouse.ru](https://releases.deckhouse.ru).
type DeckhouseRelease struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Allows or disables manual updates.
	//
	// Used only if the [`deckhouse` module](/modules/deckhouse/) is configured for manual update mode ([update.mode](/modules/deckhouse/configuration.html#parameters-update-mode) parameter is set to `Manual`). Ignored if the update mode is set to `Auto` or `AutoPatch`.
	//
	// For more information on confirming manual updates, refer to the [documentation](../../admin/configuration/update/configuration.html#manual-update-approval).
	// +optional
	// +kubebuilder:default=false
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
	// DKP version.
	// +crd-enricher:deckhouse:documentation:examples=v1.73.2
	Version string `json:"version"`
	// Marks release as a part of [canary-release](../../user/network/canary-deployment.html). This release will be delayed until the specified time. If the release is waiting to be applied, check [`.status.message`](../../admin/configuration/update/notifications.html#notification-format) for the reason.
	ApplyAfter *metav1.Time `json:"applyAfter,omitempty"`
	// A structure containing a list of requirements for installing the release. It is used by the DKP core. If the requirements are not met, the release may be skipped or blocked from installation.
	//
	// Reports on unmet requirements can be found in the field [`.status.message`](../../admin/configuration/update/notifications.html#notification-format) of the DeckhouseRelease object.
	Requirements map[string]string `json:"requirements,omitempty"`
	// Disruptive changes in the release.
	// +crd-enricher:deckhouse:documentation:deprecated=true
	Disruptions []string `json:"disruptions,omitempty"`
	// A structure containing a list of DKP changes (and modules included in the DKP) of the release.
	//
	// For more information about DKP release changelogs, refer to [Updating](../../architecture/updating.html#retrieving-the-changelog).
	// +kubebuilder:pruning:PreserveUnknownFields
	Changelog *MappedFields `json:"changelog,omitempty"`
	// Link to site with full changelog for this release.
	ChangelogLink string `json:"changelogLink,omitempty"`
}

type DeckhouseReleaseStatus struct {
	// Current status of the release.
	// +kubebuilder:validation:Enum=Pending;Deployed;Outdated;Suspended;Superseded;Skipped
	Phase string `json:"phase,omitempty"`
	// The status of the release's readiness for deployment. It makes sense only for Manual updates (`update.mode: Manual`).
	// +optional
	Approved bool `json:"approved"`
	// Time of release status change.
	TransitionTime metav1.Time `json:"transitionTime,omitempty"`
	// Detailed status or error message.
	Message string `json:"message,omitempty"`
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
