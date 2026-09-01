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

// IstioOperator describes the legacy `install.istio.io/v1alpha1` resource. The module
// does not render it anymore: Istio 1.25 is reconciled through the sail operator's
// `sailoperator.io/v1` Istio CR and newer versions run operator-free. Only the revision
// is read, to discover and clean up leftovers from retired versions.
type IstioOperator struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IstioOperatorSpec `json:"spec"`
}

type IstioOperatorSpec struct {
	Revision string `json:"revision"`
}
