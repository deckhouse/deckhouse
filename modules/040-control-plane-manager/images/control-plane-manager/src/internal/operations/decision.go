/*
Copyright 2026 Flant JSC

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

// Package operations is the ControlPlaneOperation domain: how to build an operation from a node's decision,
// how to tell whether an active operation already covers it, and how to rotate operations. It does not know
// why or when a decision is made — that is the control-plane-node domain.
package operations

import (
	"k8s.io/apimachinery/pkg/types"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

type Kind int

const (
	KindConverge Kind = iota
	KindCertRenew
	KindSignatureRenew
	KindObserve
)

type Decision struct {
	component         controlplanev1alpha1.OperationComponent
	kind              Kind
	intended          controlplanev1alpha1.Checksums
	renewCertificates bool // converge-only: reissue leaf certificates
	seedSignature     bool // converge-only: seed signature keys on first deploy
}

func ConvergeDecision(c controlplanev1alpha1.OperationComponent, intended controlplanev1alpha1.Checksums, renewCertificates, seedSignature bool) Decision {
	return Decision{component: c, kind: KindConverge, intended: intended, renewCertificates: renewCertificates, seedSignature: seedSignature}
}

func CertRenewDecision(c controlplanev1alpha1.OperationComponent, intended controlplanev1alpha1.Checksums) Decision {
	return Decision{component: c, kind: KindCertRenew, intended: intended}
}

func SignatureRenewDecision(c controlplanev1alpha1.OperationComponent, intended controlplanev1alpha1.Checksums) Decision {
	return Decision{component: c, kind: KindSignatureRenew, intended: intended}
}

func ObserveDecision(c controlplanev1alpha1.OperationComponent) Decision {
	return Decision{component: c, kind: KindObserve}
}

type NodeRef struct {
	Namespace string
	Name      string
	Type      string // control-plane.deckhouse.io/type label value
	UID       types.UID
}
