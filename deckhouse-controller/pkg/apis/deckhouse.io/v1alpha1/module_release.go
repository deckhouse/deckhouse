/*
Copyright 2023 Flant JSC

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
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModuleReleaseResource = "modulereleases"
	ModuleReleaseKind     = "ModuleRelease"

	ModuleReleasePhasePending     = "Pending"
	ModuleReleasePhaseDeployed    = "Deployed"
	ModuleReleasePhaseSuperseded  = "Superseded"
	ModuleReleasePhaseSuspended   = "Suspended"
	ModuleReleasePhaseSkipped     = "Skipped"
	ModuleReleasePhaseTerminating = "Terminating"

	ModuleReleaseApprovalAnnotation              = "modules.deckhouse.io/approved"
	ModuleReleaseAnnotationIsUpdating            = "modules.deckhouse.io/isUpdating"
	ModuleReleaseAnnotationNotified              = "modules.deckhouse.io/notified"
	ModuleReleaseAnnotationApplyNow              = "modules.deckhouse.io/apply-now"
	ModuleReleaseAnnotationRegistrySpecChanged   = "modules.deckhouse.io/registry-spec-changed"
	ModuleReleaseLabelUpdatePolicy               = "modules.deckhouse.io/update-policy"
	ModuleReleaseFinalizerExistOnFs              = "modules.deckhouse.io/exist-on-fs"
	ModuleReleaseAnnotationNotificationTimeShift = "modules.deckhouse.io/notification-time-shift"
	ModuleReleaseAnnotationForce                 = "modules.deckhouse.io/force"
	ModuleReleaseAnnotationReinstall             = "modules.deckhouse.io/reinstall"
	ModuleReleaseAnnotationChangeCause           = "modules.deckhouse.io/change-cause"

	ModuleReleaseAnnotationDryrun            = "dryrun"
	ModuleReleaseAnnotationTriggeredByDryrun = "triggered_by_dryrun"

	ModuleReleaseLabelStatus          = "status"
	ModuleReleaseLabelSource          = "source"
	ModuleReleaseLabelModule          = "module"
	ModuleReleaseLabelReleaseChecksum = "release-checksum"
)

var (
	ModuleReleaseLabelDeployed = strings.ToLower(ModuleReleasePhaseDeployed)

	ModuleReleaseGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleReleaseResource,
	}
	ModuleReleaseGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleReleaseKind,
	}
)

var _ runtime.Object = (*ModuleRelease)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ModuleRelease is a Module release object.
type ModuleRelease struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleReleaseSpec `json:"spec"`

	Status ModuleReleaseStatus `json:"status,omitempty"`
}

func (mr *ModuleRelease) GetVersion() *semver.Version {
	return semver.MustParse(mr.Spec.Version)
}

func (mr *ModuleRelease) GetModuleVersion() string {
	return "v" + semver.MustParse(mr.Spec.Version).String()
}

func (mr *ModuleRelease) GetName() string {
	return mr.Name
}

func (mr *ModuleRelease) GetApplyAfter() *time.Time {
	if mr.Spec.ApplyAfter == nil {
		return nil
	}

	return &mr.Spec.ApplyAfter.Time
}

func (mr *ModuleRelease) GetRequirements() map[string]string {
	requirements := make(map[string]string)

	if mr.Spec.Requirements == nil {
		return requirements
	}

	if len(mr.Spec.Requirements.ModuleReleasePlatformRequirements.Deckhouse) > 0 {
		requirements[DeckhouseRequirementFieldName] = mr.Spec.Requirements.ModuleReleasePlatformRequirements.Deckhouse
	}

	if len(mr.Spec.Requirements.ModuleReleasePlatformRequirements.Kubernetes) > 0 {
		requirements[KubernetesRequirementFieldName] = mr.Spec.Requirements.ModuleReleasePlatformRequirements.Kubernetes
	}

	return requirements
}

func (mr *ModuleRelease) GetModuleReleaseRequirements() *ModuleReleaseRequirements {
	return mr.Spec.Requirements
}

func (mr *ModuleRelease) GetChangelogLink() string {
	return ""
}

func (mr *ModuleRelease) GetCooldownUntil() *time.Time {
	return nil
}

func (mr *ModuleRelease) GetDisruptions() []string {
	return nil
}

func (mr *ModuleRelease) GetDisruptionApproved() bool {
	return false
}

func (mr *ModuleRelease) GetPhase() string {
	return mr.Status.Phase
}

func (mr *ModuleRelease) GetForce() bool {
	// handle deckhouse release annotation too
	v, ok := mr.Annotations[DeckhouseReleaseAnnotationForce]
	if ok && v == "true" {
		return true
	}

	v, ok = mr.Annotations[ModuleReleaseAnnotationForce]
	return ok && v == "true"
}

func (mr *ModuleRelease) GetReinstall() bool {
	return mr.Annotations[ModuleReleaseAnnotationReinstall] == "true"
}

func (mr *ModuleRelease) GetApplyNow() bool {
	return mr.Annotations[ModuleReleaseAnnotationApplyNow] == "true"
}

func (mr *ModuleRelease) GetIsUpdating() bool {
	v, ok := mr.Annotations[ModuleReleaseAnnotationIsUpdating]
	return ok && v == "true"
}

func (mr *ModuleRelease) GetNotified() bool {
	v, ok := mr.Annotations[ModuleReleaseAnnotationNotified]
	return ok && v == "true"
}

func (mr *ModuleRelease) SetApprovedStatus(val bool) {
	mr.Status.Approved = val
}

func (mr *ModuleRelease) GetSuspend() bool {
	return false
}

func (mr *ModuleRelease) GetManuallyApproved() bool {
	if approved, found := mr.ObjectMeta.Annotations[ModuleReleaseApprovalAnnotation]; found {
		value, err := strconv.ParseBool(approved)
		if err != nil {
			return false
		}

		return value
	}

	return false
}

func (mr *ModuleRelease) GetApprovedStatus() bool {
	return mr.Status.Approved
}

func (mr *ModuleRelease) GetMessage() string {
	return mr.Status.Message
}

func (mr *ModuleRelease) GetDryRun() bool {
	v, ok := mr.Annotations[ModuleReleaseAnnotationDryrun]
	return ok && v == "true"
}

func (mr *ModuleRelease) GetTriggeredByDryRun() bool {
	v, ok := mr.Annotations[ModuleReleaseAnnotationTriggeredByDryrun]
	return ok && v == "true"
}

// GetModuleSource returns module source for this release
func (mr *ModuleRelease) GetModuleSource() string {
	for _, ref := range mr.GetOwnerReferences() {
		if ref.APIVersion == ModuleSourceGVK.GroupVersion().String() && ref.Kind == ModuleSourceGVK.Kind {
			return ref.Name
		}
	}

	return mr.Labels["source"]
}

// GetModuleName returns the module's name of the release
func (mr *ModuleRelease) GetModuleName() string {
	return mr.Spec.ModuleName
}

// GetReleaseVersion returns the version of the release in the form of "vx.y.z"
func (mr *ModuleRelease) GetReleaseVersion() string {
	return "v" + semver.MustParse(mr.Spec.Version).String()
}

// GetWeight returns the weight of the related module
func (mr *ModuleRelease) GetWeight() uint32 {
	return mr.Spec.Weight
}

// GetUpdateSpec returns the optional update spec of the related release
func (mr *ModuleRelease) GetUpdateSpec() *UpdateSpec {
	return mr.Spec.UpdateSpec
}

func (c Changelog) DeepCopy() Changelog {
	if c == nil {
		return nil
	}

	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	var out Changelog
	err = json.Unmarshal(data, &out)
	if err != nil {
		panic(err)
	}

	return out
}

type ModuleReleaseRequirements struct {
	ModuleReleasePlatformRequirements `json:",inline"`
	ParentModules                     map[string]string `json:"modules,omitempty"`
}

type ModuleReleasePlatformRequirements struct {
	Deckhouse  string `json:"deckhouse,omitempty"`
	Kubernetes string `json:"kubernetes,omitempty"`
}

type ModuleReleaseSpec struct {
	ModuleName string `json:"moduleName"`
	Version    string `json:"version,omitempty"`
	Weight     uint32 `json:"weight,omitempty"`

	ApplyAfter   *metav1.Time               `json:"applyAfter,omitempty"`
	Requirements *ModuleReleaseRequirements `json:"requirements,omitempty"`
	UpdateSpec   *UpdateSpec                `json:"update,omitempty"`
	Changelog    Changelog                  `json:"changelog,omitempty"`
}

type UpdateSpec struct {
	Versions []UpdateConstraint `json:"versions,omitempty"`
}

// UpdateConstraint defines a semver range [from, to] where From is the minimal version that can upgrade directly
// to the To endpoint. Values support major.minor or full semver.
type UpdateConstraint struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ModuleReleaseStatus struct {
	Phase          string          `json:"phase,omitempty"`
	Approved       bool            `json:"approved"`
	TransitionTime metav1.Time     `json:"transitionTime,omitempty"`
	Message        string          `json:"message"`
	Size           uint32          `json:"size"`
	PullDuration   metav1.Duration `json:"pullDuration"`
}

// +kubebuilder:object:root=true

// ModuleReleaseList is a list of ModuleRelease resources
type ModuleReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleRelease `json:"items"`
}
