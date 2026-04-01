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

package pki

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

// validateCert checks whether an existing certificate is still fit for use
// given the desired configuration. It returns a non-nil error if:
//   - the certificate expires within the next 30 days
//   - the certificate's Subject or SANs no longer match the desired config
//   - the encryption algorithm no longer match the desired config
func validateCert(oldCert *x509.Certificate, newCertCfg certConfig) error {
	if certificateExpiresSoon(oldCert, 30*24*time.Hour) {
		return fmt.Errorf("expired at %s", oldCert.NotAfter.UTC().Format(time.RFC3339))
	}

	if !certificateSubjectAndSansIsEqual(oldCert, newCertCfg) {
		return fmt.Errorf("subject or SANs mismatch")
	}

	if !certificateEncryptionAlgoIsEqual(oldCert, newCertCfg) {
		return fmt.Errorf("encryption algorithm mismatch")
	}

	return nil
}

func certificateExpiresSoon(cert *x509.Certificate, durationLeft time.Duration) bool {
	return pkiutil.CertificateExpiresSoon(cert, durationLeft)
}

// certificateSubjectAndSansIsEqual checks that the existing certificate contains
// at least the Subject fields and SANs required by the desired configuration.
//
// The SAN check is intentionally one-directional (subset, not equality):
// the existing cert may have more SANs than currently configured, and that is fine —
// extra SANs do not break anything. What matters is that every SAN that is now required
// is present in the cert. The same logic applies to Organization.
func certificateSubjectAndSansIsEqual(oldCert *x509.Certificate, newCertCfg certConfig) bool {
	if oldCert.Subject.CommonName != newCertCfg.CommonName {
		return false
	}

	if !sets.New(oldCert.Subject.Organization...).Equal(sets.New(newCertCfg.Organization...)) {
		return false
	}

	certDNSNames := sets.New(oldCert.DNSNames...)
	for _, name := range newCertCfg.AltNames.DNSNames {
		if !certDNSNames.Has(name) {
			return false
		}
	}

	certIPs := make(map[string]struct{}, len(oldCert.IPAddresses))
	for _, ip := range oldCert.IPAddresses {
		certIPs[ip.String()] = struct{}{}
	}
	for _, ip := range newCertCfg.AltNames.IPs {
		if _, ok := certIPs[ip.String()]; !ok {
			return false
		}
	}

	return true
}

func certificateEncryptionAlgoIsEqual(oldCert *x509.Certificate, newCertCfg certConfig) bool {
	return detectEncryptionAlgorithm(oldCert) == newCertCfg.EncryptionAlgorithm
}

func detectEncryptionAlgorithm(cert *x509.Certificate) constants.EncryptionAlgorithmType {
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		switch pub.N.BitLen() {
		case 2048:
			return constants.EncryptionAlgorithmRSA2048
		case 3072:
			return constants.EncryptionAlgorithmRSA3072
		case 4096:
			return constants.EncryptionAlgorithmRSA4096
		}
	case *ecdsa.PublicKey:
		switch pub.Curve.Params().BitSize {
		case 256:
			return constants.EncryptionAlgorithmECDSAP256
		case 384:
			return constants.EncryptionAlgorithmECDSAP384
		}
	}

	return ""
}
