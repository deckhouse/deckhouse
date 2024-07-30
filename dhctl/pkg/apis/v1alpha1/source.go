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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ModuleSource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of an ModuleSource.
	Spec ModuleSourceSpec `json:"spec"`

	// Status of an ModuleSource.
	Status ModuleSourceStatus `json:"status,omitempty"`
}

type ModuleSourceSpec struct {
	Registry       ModuleSourceSpecRegistry `json:"registry"`
	ReleaseChannel string                   `json:"releaseChannel"`
}

type ModuleSourceSpecRegistry struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	DockerCFG string `json:"dockerCfg"`
	CA        string `json:"ca"`
}

type ModuleSourceStatus struct {
	SyncTime         time.Time     `json:"syncTime"`
	ModulesCount     int           `json:"modulesCount"`
	AvailableModules []string      `json:"availableModules"`
	Msg              string        `json:"message"`
	ModuleErrors     []ModuleError `json:"moduleErrors"`
}

type ModuleError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}
