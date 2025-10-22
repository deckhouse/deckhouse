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

package crd

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type IstioOperator struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec IstioOperatorSpec `json:"spec"`

	Status IstioOperatorStatus `json:"status"`
}

type IstioOperatorSpec struct {
	Revision string `json:"revision"`
}

type IstioOperatorStatus struct {
	Status          string                       `json:"status"`
	ComponentStatus IstioOperatorComponentStatus `json:"componentStatus"`
}

type IstioOperatorComponentStatus struct {
	Pilot IstioOperatorComponentStatusDetails `json:"Pilot"`
}

type IstioOperatorComponentStatusDetails struct {
	Error  string `json:"error"`
	Status string `json:"status"`
}
