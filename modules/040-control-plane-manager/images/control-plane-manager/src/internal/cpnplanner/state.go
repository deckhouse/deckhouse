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

package cpnplanner

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// componentState is the intended (from spec) versus actual (from status) state of a single component.
type componentState struct {
	component       controlplanev1alpha1.OperationComponent
	intended        controlplanev1alpha1.Checksums // Config, PKI, CA(=global)
	actual          controlplanev1alpha1.Checksums // Config, PKI, CA(per-component)
	certExpiry      time.Time                      // earliest observed leaf-certificate expiry (excludes signature); zero if unobserved
	signatureExpiry time.Time                      // observed signature-key expiry (kube-apiserver, CSE); zero if unobserved
	lastObserve     time.Time                      // last successful certificate observation; zero if never
}

func (s componentState) inSync() bool {
	return s.intended.Config == s.actual.Config &&
		(!s.component.HasPKI() || s.intended.PKI == s.actual.PKI) &&
		s.intended.CA == s.actual.CA
}

func (s componentState) certsChanged() bool {
	return s.intended.PKI != s.actual.PKI || s.intended.CA != s.actual.CA
}

func (s componentState) needsConverge() bool {
	return !s.inSync()
}

func (s componentState) needsObserve() bool {
	if s.actual.Config == "" {
		return false // not deployed yet, nothing to observe
	}
	return s.lastObserve.IsZero() || time.Since(s.lastObserve) > constants.CertObserveInterval
}

// needsCertRenew reports whether leaf certificates expire soon and no converge is already reissuing them.
func (s componentState) needsCertRenew() bool {
	if s.certsChanged() {
		return false
	}
	return !s.certExpiry.IsZero() && time.Until(s.certExpiry) < constants.CertRenewalThreshold
}

// needsSignatureRenew reports whether the kube-apiserver signature key expires soon (CSE builds only).
func (s componentState) needsSignatureRenew() bool {
	if !constants.SignatureEnabled() || s.component != controlplanev1alpha1.OperationComponentKubeAPIServer {
		return false
	}
	return !s.signatureExpiry.IsZero() && time.Until(s.signatureExpiry) < constants.SignatureRenewalThreshold
}

// needsSignatureBootstrap reports whether the first kube-apiserver deploy must seed the signature keys (CSE builds only).
func (s componentState) needsSignatureBootstrap() bool {
	return constants.SignatureEnabled() &&
		s.component == controlplanev1alpha1.OperationComponentKubeAPIServer &&
		s.actual.Config == ""
}

// computeComponentStates pairs intended (spec + global CA) with actual (status) per component, in a stable order.
func computeComponentStates(cpn *controlplanev1alpha1.ControlPlaneNode) []componentState {
	type entry struct {
		component controlplanev1alpha1.OperationComponent
		spec      controlplanev1alpha1.Checksums
	}
	entries := []entry{
		{controlplanev1alpha1.OperationComponentEtcd, cpn.Spec.Components.Etcd.Checksums},
		{controlplanev1alpha1.OperationComponentKubeAPIServer, cpn.Spec.Components.KubeAPIServer.Checksums},
		{controlplanev1alpha1.OperationComponentKubeControllerManager, cpn.Spec.Components.KubeControllerManager.Checksums},
		{controlplanev1alpha1.OperationComponentKubeScheduler, cpn.Spec.Components.KubeScheduler.Checksums},
	}

	states := make([]componentState, 0, len(entries))
	for _, e := range entries {
		if e.spec.Config == "" && e.spec.PKI == "" {
			continue // component not configured for this node
		}
		st := componentState{
			component: e.component,
			intended: controlplanev1alpha1.Checksums{
				Config: e.spec.Config,
				PKI:    e.spec.PKI,
				CA:     cpn.Spec.CAChecksum,
			},
		}
		if cs := cpn.Status.Components.Component(e.component); cs != nil {
			st.actual = cs.Checksums
			st.certExpiry = minExpirationExcluding(cs.CertificatesExpirationTime, constants.SignatureExpirationKey)
			st.signatureExpiry = cs.CertificatesExpirationTime[constants.SignatureExpirationKey].Time
			st.lastObserve = cs.LastCertObserveTime.Time
		}
		states = append(states, st)
	}
	return states
}

func minExpiration(dates map[string]metav1.Time) time.Time {
	return minExpirationExcluding(dates, "")
}

func minExpirationExcluding(dates map[string]metav1.Time, exclude string) time.Time {
	var min time.Time
	for key, t := range dates {
		if key == exclude {
			continue
		}
		if min.IsZero() || t.Time.Before(min) {
			min = t.Time
		}
	}
	return min
}
