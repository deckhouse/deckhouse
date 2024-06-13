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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
)

var (
	ModuleUpdatePolicyGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "moduleupdatepolicies",
	}
	ModuleUpdatePolicyGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModuleUpdatePolicy",
	}
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleUpdatePolicy source
type ModuleUpdatePolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleUpdatePolicySpec `json:"spec"`
}

type ModuleUpdatePolicySpec struct {
	Update                ModuleUpdatePolicySpecUpdate          `json:"update"`
	ReleaseChannel        string                                `json:"releaseChannel"`
	ModuleReleaseSelector ModuleUpdatePolicySpecReleaseSelector `json:"moduleReleaseSelector"`
}

type ModuleUpdatePolicySpecUpdate struct {
	Mode    string         `json:"mode"`
	Windows update.Windows `json:"windows"`
}

type ModuleUpdatePolicySpecReleaseSelector struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleUpdatePolicyList is a list of ModuleUpdatePolicy resources
type ModuleUpdatePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleUpdatePolicy `json:"items"`
}

func NewModuleUpdatePolicySpecContainer(spec *ModuleUpdatePolicySpec) *ModuleUpdatePolicySpecContainer {
	return &ModuleUpdatePolicySpecContainer{spec: spec}
}

type ModuleUpdatePolicySpecContainer struct {
	spec *ModuleUpdatePolicySpec
	lock sync.Mutex
}

func (c *ModuleUpdatePolicySpecContainer) Set(spec *ModuleUpdatePolicySpec) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.spec.ReleaseChannel = spec.ReleaseChannel
	c.spec.Update.Mode = spec.Update.Mode
	c.spec.Update.Windows = spec.Update.Windows
}

func (c *ModuleUpdatePolicySpecContainer) Get() *ModuleUpdatePolicySpec {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.spec.DeepCopy()
}
