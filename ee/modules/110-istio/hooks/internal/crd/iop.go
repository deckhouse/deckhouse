/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	Status string `json:"status"`
}
