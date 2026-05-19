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

package controlplaneoperation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// observeCertExpirationsForStaticPod reads leaf cert and kubeconfig client cert expirations for one static pod component.
func observeCertExpirationsForStaticPod(component controlplanev1alpha1.OperationComponent, kubeconfigDir string, logger *log.Logger) (controlplanev1alpha1.ObservedComponentState, bool) {
	if !component.IsStaticPodComponent() {
		return controlplanev1alpha1.ObservedComponentState{}, false
	}
	deps := componentDeps(component)

	certExpiry := make(map[string]metav1.Time)

	if leafNames := deps.leafCertFiles(); len(leafNames) > 0 {
		expirations, err := pki.ListCertificateExpirations(
			pki.WithCertificatesDir(constants.KubernetesPkiPath),
			pki.WithLeafCertificates(leafNames...),
			pki.WithIgnoreReadErrors(),
		)
		for _, e := range joinedErrors(err) {
			logger.Warn("cannot read cert expiration", log.Err(e))
		}
		for _, exp := range expirations {
			certExpiry[exp.Name+".crt"] = metav1.NewTime(exp.NotAfter)
		}
	}

	if len(deps.KubeconfigFiles) > 0 {
		expirations, err := kubeconfig.ListClientCertificateExpirations(
			kubeconfig.WithKubeconfigDir(kubeconfigDir),
			kubeconfig.WithFiles(deps.KubeconfigFiles...),
			kubeconfig.WithIgnoreReadErrors(),
		)
		for _, e := range joinedErrors(err) {
			logger.Warn("cannot read kubeconfig cert expiration", log.Err(e))
		}
		for _, exp := range expirations {
			certExpiry[string(exp.File)] = metav1.NewTime(exp.NotAfter)
		}
	}

	if len(certExpiry) == 0 {
		return controlplanev1alpha1.ObservedComponentState{}, true
	}
	return controlplanev1alpha1.ObservedComponentState{
		CertificatesExpirationDate: certExpiry,
	}, true
}

// joinedErrors unwraps an errors.Join result into individual errors. If err is
// nil it returns nil; if err is a plain (non-joined) error it is returned as a
// single-element slice.
func joinedErrors(err error) []error {
	if err == nil {
		return nil
	}
	type multiErr interface{ Unwrap() []error }
	if me, ok := err.(multiErr); ok {
		return me.Unwrap()
	}
	return []error{err}
}
