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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// readCertExpiration reads a PEM certificate file and returns its NotAfter time.
func readCertExpiration(certPath string) (metav1.Time, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("read %s: %w", certPath, err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return metav1.Time{}, fmt.Errorf("no PEM block in %s", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("parse %s: %w", certPath, err)
	}

	return metav1.NewTime(cert.NotAfter), nil
}

// readKubeconfigCertExpiration extracts client certificate NotAfter from a kubeconfig file.
func readKubeconfigCertExpiration(kubeconfigPath string) (metav1.Time, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("load kubeconfig %s: %w", kubeconfigPath, err)
	}

	ctx, ok := config.Contexts[config.CurrentContext]
	if !ok {
		return metav1.Time{}, fmt.Errorf("current context not found in %s", kubeconfigPath)
	}

	authInfo, ok := config.AuthInfos[ctx.AuthInfo]
	if !ok || len(authInfo.ClientCertificateData) == 0 {
		return metav1.Time{}, fmt.Errorf("no client-certificate-data in %s", kubeconfigPath)
	}

	block, _ := pem.Decode(authInfo.ClientCertificateData)
	if block == nil {
		return metav1.Time{}, fmt.Errorf("no PEM block in client-certificate-data of %s", kubeconfigPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("parse client cert from %s: %w", kubeconfigPath, err)
	}

	return metav1.NewTime(cert.NotAfter), nil
}

// observeCertExpirationsForStaticPod reads leaf cert and kubeconfig client cert expirations for one static pod component.
func observeCertExpirationsForStaticPod(component controlplanev1alpha1.OperationComponent, kubeconfigDir string, logger *log.Logger) (state controlplanev1alpha1.ObservedComponentState, ok bool) {
	if !component.IsStaticPodComponent() {
		return controlplanev1alpha1.ObservedComponentState{}, false
	}
	deps := componentDeps(component)

	certExpiry := make(map[string]metav1.Time)

	for _, leafName := range deps.leafCertFiles() {
		baseName := string(leafName)
		certPath := filepath.Join(constants.KubernetesPkiPath, baseName+".crt")
		expiry, err := readCertExpiration(certPath)
		if err != nil {
			logger.Warn("cannot read cert expiration",
				slog.String("cert", certPath), log.Err(err))
			continue
		}
		certExpiry[baseName+".crt"] = expiry
	}

	for _, file := range deps.KubeconfigFiles {
		kubeconfigPath := filepath.Join(kubeconfigDir, string(file))
		expiry, err := readKubeconfigCertExpiration(kubeconfigPath)
		if err != nil {
			logger.Warn("cannot read kubeconfig cert expiration",
				slog.String("kubeconfig", kubeconfigPath), log.Err(err))
			continue
		}
		certExpiry[string(file)] = expiry
	}

	if len(certExpiry) == 0 {
		return controlplanev1alpha1.ObservedComponentState{}, true
	}
	return controlplanev1alpha1.ObservedComponentState{
		CertificatesExpirationDate: certExpiry,
	}, true
}
