/*
Copyright 2025 Flant JSC

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

package csr

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	certificatesv1 "k8s.io/api/certificates/v1"
)

// kubeletServingRequiredUsages lists the key usages required for a kubelet serving
// certificate when using RSA keys.
var kubeletServingRequiredUsages = []certificatesv1.KeyUsage{
	certificatesv1.UsageKeyEncipherment,
	certificatesv1.UsageDigitalSignature,
	certificatesv1.UsageServerAuth,
}

// kubeletServingRequiredUsagesNoRSA lists the key usages required for a kubelet
// serving certificate when using non-RSA keys (e.g. ECDSA).
var kubeletServingRequiredUsagesNoRSA = []certificatesv1.KeyUsage{
	certificatesv1.UsageDigitalSignature,
	certificatesv1.UsageServerAuth,
}

// parseCSR decodes the PEM-encoded CSR request and returns the parsed x509
// CertificateRequest.
func parseCSR(obj *certificatesv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	pemBytes := obj.Spec.Request
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	cr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cr, nil
}

// validateNodeServingCert validates that the CSR is a legitimate kubelet serving
// certificate request per Kubernetes conventions.
func validateNodeServingCert(csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return fmt.Errorf("org does not match: %s", x509cr.Subject.Organization)
	}

	if len(x509cr.IPAddresses)+len(x509cr.DNSNames) < 1 {
		return fmt.Errorf("field IPAddresses or DNSNames must be set")
	}

	if len(x509cr.EmailAddresses) > 0 {
		return fmt.Errorf("field EmailAddresses is present")
	}

	if len(x509cr.URIs) > 0 {
		return fmt.Errorf("field URIs is present")
	}

	if !hasExactUsages(csr, kubeletServingRequiredUsages) && !hasExactUsages(csr, kubeletServingRequiredUsagesNoRSA) {
		return fmt.Errorf("usage does not match")
	}

	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return fmt.Errorf("CN does not start with 'system:node:': %s", x509cr.Subject.CommonName)
	}

	if csr.Spec.Username != x509cr.Subject.CommonName {
		return fmt.Errorf("x509 CN %q doesn't match CSR username %q", x509cr.Subject.CommonName, csr.Spec.Username)
	}

	return nil
}

// hasExactUsages checks that the CSR has exactly the given set of key usages.
func hasExactUsages(csr *certificatesv1.CertificateSigningRequest, usages []certificatesv1.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}

	usageMap := make(map[certificatesv1.KeyUsage]struct{}, len(usages))
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

// isAlreadyApprovedOrDenied checks whether the CSR already has an Approved or Denied condition.
func isAlreadyApprovedOrDenied(csr *certificatesv1.CertificateSigningRequest) bool {
	for _, c := range csr.Status.Conditions {
		if c.Type == certificatesv1.CertificateApproved || c.Type == certificatesv1.CertificateDenied {
			return true
		}
	}
	return false
}
