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
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	PhasePending         = "Pending"
	PhasePolicyUndefined = "PolicyUndefined"
	PhaseDeployed        = "Deployed"
	PhaseSuperseded      = "Superseded"
	PhaseSuspended       = "Suspended"
	PhaseSkipped         = "Skipped"

	approvalAnnotation = "modules.deckhouse.io/approved"
)

var (
	ModuleReleaseGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "modulereleases",
	}
	ModuleReleaseGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModuleRelease",
	}
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	return mr.Spec.Version
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
	return false
}

func (mr *ModuleRelease) GetApplyNow() bool {
	return mr.Annotations["release.deckhouse.io/apply-now"] == "true"
}

func (mr *ModuleRelease) SetApprovedStatus(val bool) {
	mr.Status.Approved = val
}

func (mr *ModuleRelease) GetSuspend() bool {
	return false
}

func (mr *ModuleRelease) GetManuallyApproved() bool {
	if approved, found := mr.ObjectMeta.Annotations[approvalAnnotation]; found {
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
	return "v" + mr.Spec.Version.String()
}

// GetWeight returns the weight of the related module
func (mr *ModuleRelease) GetWeight() uint32 {
	return mr.Spec.Weight
}

type Changelog map[string]any

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

type ModuleReleaseSpec struct {
	ModuleName string          `json:"moduleName"`
	Version    *semver.Version `json:"version,omitempty"`
	Weight     uint32          `json:"weight,omitempty"`

	ApplyAfter   *metav1.Time      `json:"applyAfter,omitempty"`
	Requirements map[string]string `json:"requirements,omitempty"`
	Changelog    Changelog         `json:"changelog,omitempty"`
}

type ModuleReleaseStatus struct {
	Phase          string          `json:"phase,omitempty"`
	Approved       bool            `json:"approved"`
	TransitionTime metav1.Time     `json:"transitionTime,omitempty"`
	Message        string          `json:"message"`
	Size           uint32          `json:"size"`
	PullDuration   metav1.Duration `json:"pullDuration"`
}

type moduleReleaseKind struct{}

func (in *ModuleReleaseStatus) GetObjectKind() schema.ObjectKind {
	return &moduleReleaseKind{}
}

func (f *moduleReleaseKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *moduleReleaseKind) GroupVersionKind() schema.GroupVersionKind {
	return ModuleReleaseGVK
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleReleaseList is a list of ModuleRelease resources
type ModuleReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleRelease `json:"items"`
}
