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
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// observeCertExpirationsForStaticPod reads leaf cert and kubeconfig client cert expirations for one static pod component.
func observeCertExpirationsForStaticPod(component controlplanev1alpha1.OperationComponent, kubeconfigDir string, logger *log.Logger) (controlplanev1alpha1.ObservedComponentState, bool, error) {
	if !component.IsStaticPodComponent() {
		return controlplanev1alpha1.ObservedComponentState{}, false, nil
	}
	deps := componentDeps(component)

	certExpiry := make(map[string]metav1.Time)
	var readErrs []error

	if leafNames := deps.leafCertFiles(); len(leafNames) > 0 {
		report, err := pki.ListCertificateExpirations(
			pki.WithCertificatesDir(constants.KubernetesPkiPath),
			pki.WithLeafCertificates(leafNames...),
		)
		if err != nil {
			logger.Warn("cannot list cert expirations", "error", err)
			readErrs = append(readErrs, err)
		}
		for _, e := range report.Entries {
			if e.Err != nil {
				logger.Warn("cannot read cert expiration", "name", e.Name, "path", e.Path, "error", e.Err)
				readErrs = append(readErrs, fmt.Errorf("cert %q: %w", e.Name, e.Err))
				continue
			}
			certExpiry[e.Name+".crt"] = metav1.NewTime(e.NotAfter)
		}
	}

	if len(deps.KubeconfigFiles) > 0 {
		report := kubeconfig.ListClientCertificateExpirations(
			kubeconfig.WithKubeconfigDir(kubeconfigDir),
			kubeconfig.WithFiles(deps.KubeconfigFiles...),
		)
		for _, e := range report.Entries {
			if e.Err != nil {
				logger.Warn("cannot read kubeconfig cert expiration", "file", e.File, "path", e.Path, "error", e.Err)
				readErrs = append(readErrs, fmt.Errorf("kubeconfig %q: %w", e.File, e.Err))
				continue
			}
			certExpiry[string(e.File)] = metav1.NewTime(e.NotAfter)
		}
	}

	state := controlplanev1alpha1.ObservedComponentState{}
	if len(certExpiry) > 0 {
		state.CertificatesExpirationDate = certExpiry
	}
	return state, true, errors.Join(readErrs...)
}
