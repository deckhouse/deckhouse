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
	"k8s.io/apimachinery/pkg/runtime"
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

// +kubebuilder:pruning:XPreserveUnknownFields
type Changelog runtime.RawExtension // map[string]any

// MarshalJSON implements json.Marshaler
func (v Changelog) MarshalJSON() ([]byte, error) {
	if v.Raw != nil {
		return v.Raw, nil
	}
	return []byte("{}"), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (v *Changelog) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	v.Raw = make([]byte, len(data))
	copy(v.Raw, data)
	return nil
}

func (v *Changelog) DeepCopy() *Changelog {
	if v == nil {
		return nil
	}

	out := new(Changelog)
	v.DeepCopyInto(out)

	return out
}

func (v *Changelog) DeepCopyInto(out *Changelog) {
	if v.Raw != nil {
		out.Raw = make([]byte, len(v.Raw))
		copy(out.Raw, v.Raw)
	} else {
		out.Raw = nil
	}
	if v.Object != nil {
		out.Object = v.Object.DeepCopyObject()
	} else {
		out.Object = nil
	}
}

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
