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

package bashiblecontext

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	csrutil "k8s.io/client-go/util/certificate/csr"
)

const (
	// certCommonName / certRoleName mirror the former create_rbac_and_certificate
	// _for_kubernetes_api_proxy hook: the cert authenticates as user
	// "kubernetes-api-proxy" in group "node-manager:kubernetes-api-proxy", the
	// group the kubernetes-api-proxy ClusterRoleBinding grants.
	certCommonName = "kubernetes-api-proxy"
	certRoleName   = "node-manager:kubernetes-api-proxy"

	// certOutdatedDuration rotates the cert once less than half its 10-year life
	// remains, identical to the hook's threshold.
	certOutdatedDuration = (24 * time.Hour) * 365 / 2

	csrWaitTimeout = time.Minute
)

// certExpirationSeconds requests a 10-year cert from the signer.
var certExpirationSeconds = int32((time.Hour * 24 * 365 * 10).Seconds())

// ensureCertificate issues (or re-issues) the discovery cert into the
// kube-system/kubernetes-api-proxy-discovery-cert Secret when it is absent or the
// stored cert is within certOutdatedDuration of expiry. It runs at the top of
// Reconcile, before the blob is assembled, so readAPIServerProxyCerts below always
// finds the Secret in the same loop — closing the fresh-cluster window where the
// blob would render with a nil apiserverProxyCerts. In steady state it is a cheap
// Get that returns early.
func (c *Controller) ensureCertificate(ctx context.Context, logger logr.Logger) error {
	secret, err := c.clientset.CoreV1().Secrets(kubeSystemNS).Get(ctx, apiProxyCertSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get %s/%s: %w", kubeSystemNS, apiProxyCertSecretName, err)
	}

	if err == nil {
		if crt := secret.Data["crt"]; len(crt) > 0 {
			cert, perr := parseCertificate(crt)
			if perr == nil && time.Until(cert.NotAfter) >= certOutdatedDuration {
				return nil
			}
		}
	}

	logger.Info("issuing kubernetes-api-proxy discovery certificate")
	crtPEM, keyPEM, err := c.issueCertificate(ctx)
	if err != nil {
		return err
	}
	return c.writeCertSecret(ctx, crtPEM, keyPEM)
}

// issueCertificate runs the CSR flow against the kube-apiserver-client signer:
// generate a key + CSR, submit, self-approve, wait for the signed cert, then
// delete the CSR. This is the controller-runtime port of tls_certificate.IssueCertificate.
func (c *Controller) issueCertificate(ctx context.Context) ([]byte, []byte, error) {
	csrPEM, keyPEM, err := generateCSR(certCommonName, []string{certRoleName})
	if err != nil {
		return nil, nil, fmt.Errorf("generate CSR: %w", err)
	}

	csrClient := c.clientset.CertificatesV1().CertificateSigningRequests()

	// Drop a stale CSR left by a previous partial run (same fixed name).
	_ = csrClient.Delete(ctx, certCommonName, metav1.DeleteOptions{})

	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{Name: certCommonName},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:           csrPEM,
			SignerName:        certificatesv1.KubeAPIServerClientSignerName,
			Usages:            []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth},
			ExpirationSeconds: &certExpirationSeconds,
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

	waitCtx, cancel := context.WithTimeout(ctx, csrWaitTimeout)
	defer cancel()

	crtPEM, err := csrutil.WaitForCertificate(waitCtx, c.clientset, req.Name, req.UID)
	if err != nil {
		return nil, nil, fmt.Errorf("wait for signed certificate: %w", err)
	}

	_ = csrClient.Delete(ctx, certCommonName, metav1.DeleteOptions{})

	return crtPEM, keyPEM, nil
}

// writeCertSecret upserts the crt/key into the discovery cert Secret, preserving
// the hook's labels so ownership/reporting is unchanged.
func (c *Controller) writeCertSecret(ctx context.Context, crtPEM, keyPEM []byte) error {
	labels := map[string]string{
		"heritage": "deckhouse",
		"module":   "node-manager",
	}
	data := map[string][]byte{
		"crt": crtPEM,
		"key": keyPEM,
	}

	secrets := c.clientset.CoreV1().Secrets(kubeSystemNS)
	existing, err := secrets.Get(ctx, apiProxyCertSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = secrets.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apiProxyCertSecretName,
				Namespace: kubeSystemNS,
				Labels:    labels,
			},
			Data: data,
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create %s/%s: %w", kubeSystemNS, apiProxyCertSecretName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get %s/%s: %w", kubeSystemNS, apiProxyCertSecretName, err)
	}

	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range labels {
		existing.Labels[k] = v
	}
	existing.Data = data
	if _, err := secrets.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update %s/%s: %w", kubeSystemNS, apiProxyCertSecretName, err)
	}
	return nil
}

// generateCSR builds an ECDSA P-256 key and a client CSR whose Organization is
// the RBAC group (the signer maps it to the "node-manager:kubernetes-api-proxy"
// group). It returns PEM-encoded CSR and PKCS#8 key.
func generateCSR(commonName string, organizations []string) (csrPEM, keyPEM []byte, err error) {
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
	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return csrPEM, keyPEM, nil
}

// parseCertificate decodes the first PEM block of a cert to read NotAfter.
func parseCertificate(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseCertificate(block.Bytes)
}
