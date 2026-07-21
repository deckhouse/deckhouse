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

package capi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	csrutil "k8s.io/client-go/util/certificate/csr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// capiControllerManagerName is the kubeconfig user identity capi-controller-manager
	// authenticates as; capiManagerRole is the RBAC group the client cert is bound to.
	capiControllerManagerName = "capi-controller-manager"
	capiManagerRole           = "d8:node-manager:capi-controller-manager:manager-role"

	// kubeconfigCertRenewBefore rotates the 180-day cert once less than half its life remains.
	kubeconfigCertRenewBefore = (24 * time.Hour) * 90

	kubeconfigCSRWaitTimeout = time.Minute
)

var kubeconfigCertExpirationSeconds = int32((180 * 24 * time.Hour).Seconds())

// ensureKubeconfigSecret creates/refreshes the <clusterName>-kubeconfig Secret; no-op while fresh.
func (r *ClusterReconciler) ensureKubeconfigSecret(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)

	secretName := fmt.Sprintf("%s-kubeconfig", clusterName)

	existing := &corev1.Secret{}
	getErr := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: capiNamespace}, existing)
	if getErr == nil && kubeconfigCertFresh(existing.Data["value"]) {
		return nil
	}
	if getErr != nil && client.IgnoreNotFound(getErr) != nil {
		return fmt.Errorf("get %s/%s: %w", capiNamespace, secretName, getErr)
	}

	host, caData, err := r.apiServerEndpoint()
	if err != nil {
		return err
	}

	logger.Info("issuing capi-controller-manager kubeconfig certificate", "cluster", clusterName)
	crtPEM, keyPEM, err := r.issueCAPIClientCert(ctx, clusterName)
	if err != nil {
		return err
	}

	kubeconfigYAML, err := buildKubeconfigYAML(clusterName, host, caData, keyPEM, crtPEM)
	if err != nil {
		return err
	}

	return r.writeKubeconfigSecret(ctx, secretName, clusterName, kubeconfigYAML)
}

// apiServerEndpoint returns the API server URL and CA bundle from the manager's rest config.
func (r *ClusterReconciler) apiServerEndpoint() (string, []byte, error) {
	if r.RestConfig == nil {
		return "", nil, fmt.Errorf("rest config is not initialised")
	}
	caData := r.RestConfig.CAData
	if len(caData) == 0 && r.RestConfig.CAFile != "" {
		b, err := os.ReadFile(r.RestConfig.CAFile)
		if err != nil {
			return "", nil, fmt.Errorf("read CA file %s: %w", r.RestConfig.CAFile, err)
		}
		caData = b
	}
	return r.RestConfig.Host, caData, nil
}

func (r *ClusterReconciler) issueCAPIClientCert(ctx context.Context, clusterName string) ([]byte, []byte, error) {
	csrPEM, keyPEM, err := generateClientCSR(capiControllerManagerName, []string{capiManagerRole})
	if err != nil {
		return nil, nil, fmt.Errorf("generate CSR: %w", err)
	}

	csrName := fmt.Sprintf("capi-controller-manager-%s-kubeconfig", clusterName)
	csrClient := r.Clientset.CertificatesV1().CertificateSigningRequests()

	// Drop a stale CSR left by a previous partial run (fixed name per cluster).
	_ = csrClient.Delete(ctx, csrName, metav1.DeleteOptions{})

	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{Name: csrName},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:           csrPEM,
			SignerName:        certificatesv1.KubeAPIServerClientSignerName,
			Usages:            []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth},
			ExpirationSeconds: &kubeconfigCertExpirationSeconds,
		},
	}

	req, err := csrClient.Create(ctx, csr, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("create CertificateSigningRequest: %w", err)
	}

	req.Status.Conditions = append(req.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:    certificatesv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "NodeControllerApprove",
		Message: "This CSR was approved by node-controller.",
	})
	if _, err := csrClient.UpdateApproval(ctx, req.Name, req, metav1.UpdateOptions{}); err != nil {
		return nil, nil, fmt.Errorf("approve CertificateSigningRequest: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, kubeconfigCSRWaitTimeout)
	defer cancel()

	crtPEM, err := csrutil.WaitForCertificate(waitCtx, r.Clientset, req.Name, req.UID)
	if err != nil {
		return nil, nil, fmt.Errorf("wait for signed certificate: %w", err)
	}

	_ = csrClient.Delete(ctx, csrName, metav1.DeleteOptions{})

	return crtPEM, keyPEM, nil
}

func (r *ClusterReconciler) writeKubeconfigSecret(ctx context.Context, secretName, clusterName string, kubeconfigYAML []byte) error {
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: capiNamespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": clusterName,
			},
		},
		Type: "cluster.x-k8s.io/secret",
		Data: map[string][]byte{"value": kubeconfigYAML},
	}

	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: capiNamespace}, existing)
	if apierrors.IsNotFound(err) {
		if cErr := r.Client.Create(ctx, desired); cErr != nil {
			return fmt.Errorf("create %s/%s: %w", capiNamespace, secretName, cErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get %s/%s: %w", capiNamespace, secretName, err)
	}

	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	existing.Labels["cluster.x-k8s.io/cluster-name"] = clusterName
	existing.Type = desired.Type
	existing.Data = desired.Data
	if uErr := r.Client.Update(ctx, existing); uErr != nil {
		return fmt.Errorf("update %s/%s: %w", capiNamespace, secretName, uErr)
	}
	return nil
}

// buildKubeconfigYAML assembles a kubeconfig for capi-controller-manager.
func buildKubeconfigYAML(clusterName, host string, caData, keyPEM, crtPEM []byte) ([]byte, error) {
	userName := capiControllerManagerName
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	cfg := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   host,
				CertificateAuthorityData: caData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			userName: {
				ClientKeyData:         keyPEM,
				ClientCertificateData: crtPEM,
			},
		},
		CurrentContext: contextName,
	}

	return clientcmd.Write(cfg)
}

// kubeconfigCertFresh reports whether the embedded client cert has more than
// kubeconfigCertRenewBefore of validity left.
func kubeconfigCertFresh(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	cfg, err := clientcmd.Load(value)
	if err != nil {
		return false
	}
	auth, ok := cfg.AuthInfos[capiControllerManagerName]
	if !ok || len(auth.ClientCertificateData) == 0 {
		return false
	}
	block, _ := pem.Decode(auth.ClientCertificateData)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return time.Until(cert.NotAfter) > kubeconfigCertRenewBefore
}

func generateClientCSR(commonName string, organizations []string) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: commonName, Organization: organizations},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}, key)
	if err != nil {
		return nil, nil, err
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return csrPEM, keyPEM, nil
}
