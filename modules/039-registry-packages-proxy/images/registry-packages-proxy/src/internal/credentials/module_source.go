// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credentials

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var moduleSourceGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1alpha1",
	Resource: "modulesources",
}

type ModuleSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleSourceSpec `json:"spec"`
}

type ModuleSourceSpec struct {
	Registry ModuleSourceSpecRegistry `json:"registry"`
}

type ModuleSourceSpecRegistry struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	DockerCFG []byte `json:"dockerCfg"`
	CA        string `json:"ca"`
}
