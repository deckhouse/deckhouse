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
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// observeCertFiles maps component names to cert file base names (without .crt) relative to pki dir.
var observeCertFiles = map[string][]string{
	"etcd": {
		"etcd/ca",
		"etcd/server",
		"etcd/peer",
		"etcd/healthcheck-client",
		"apiserver-etcd-client",
	},
	"kube-apiserver": {
		"ca",
		"apiserver",
		"apiserver-kubelet-client",
		"front-proxy-ca",
		"front-proxy-client",
	},
}

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

// execObserve collects certificate expiration dates from disk and writes them to CPO status.
func execObserve(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	observedState := make(map[string]controlplanev1alpha1.ObservedComponentState)

	// PKI expirations
	for compName, certFiles := range observeCertFiles {
		certExpiry := make(map[string]metav1.Time)
		for _, baseName := range certFiles {
			certPath := filepath.Join(constants.KubernetesPkiPath, baseName+".crt")
			expiry, err := readCertExpiration(certPath)
			if err != nil {
				logger.Warn("cannot read cert expiration",
					slog.String("cert", certPath), log.Err(err))
				continue
			}
			certExpiry[baseName+".crt"] = expiry
		}
		if len(certExpiry) > 0 {
			observedState[compName] = controlplanev1alpha1.ObservedComponentState{
				CertificatesExpiration: certExpiry,
			}
		}
	}

	// kubeconfig expirations
	kubeconfigDir := kubeconfigDirPath()
	for component, compName := range controlplanev1alpha1.ComponentRegistry() {
		kubeconfigFiles := kubeconfigFilesForComponent(component)
		if len(kubeconfigFiles) == 0 {
			continue
		}
		state := observedState[compName]
		if state.CertificatesExpiration == nil {
			state.CertificatesExpiration = make(map[string]metav1.Time)
		}
		for _, file := range kubeconfigFiles {
			kubeconfigPath := filepath.Join(kubeconfigDir, string(file))
			expiry, err := readKubeconfigCertExpiration(kubeconfigPath)
			if err != nil {
				logger.Warn("cannot read kubeconfig cert expiration",
					slog.String("kubeconfig", kubeconfigPath), log.Err(err))
				continue
			}
			state.CertificatesExpiration[string(file)] = expiry
		}
		if len(state.CertificatesExpiration) > 0 {
			observedState[compName] = state
		}
	}

	original := cc.op.DeepCopy()
	cc.op.Status.ObservedState = observedState
	if err := cc.r.client.Status().Patch(ctx, cc.op, client.MergeFrom(original)); err != nil {
		return reconcile.Result{}, fmt.Errorf("patch observed state: %w", err)
	}

	logger.Info("observed certificate expiration", slog.Int("components", len(observedState)))
	return reconcile.Result{}, nil
}
