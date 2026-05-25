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
	"crypto/x509"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	certutil "k8s.io/client-go/util/cert"
)

func certConfigFromX509(cert *x509.Certificate) certConfig {
	return certConfig{
		Config: certutil.Config{
			CommonName:   cert.Subject.CommonName,
			Organization: cert.Subject.Organization,
			AltNames: certutil.AltNames{
				DNSNames: cert.DNSNames,
				IPs:      cert.IPAddresses,
			},
			Usages: cert.ExtKeyUsage,
		},
		EncryptionAlgorithm: pkiutil.DetectEncryptionAlgorithm(cert),
	}
}

func caForLeaf(name LeafCertName) (RootCertName, bool) {
	for caName, leafNames := range defaultCertTreeScheme {
		for _, leafName := range leafNames {
			if leafName == name {
				return caName, true
			}
		}
	}
	return "", false
}

// RenewLeafCert renews a leaf certificate by re-signing it with the same private key.
// All Subject/SAN/Usage/Algorithm fields are preserved from the current certificate file.
// The new certificate is issued with constants.CertificateValidityPeriod (1 year).
// Sentinel errors:
//   - *CertMissingError  — leaf cert file absent (skippable)
//   - *CAExternalError   — CA key absent (skippable)
//   - *CAExpiredError    — CA cert expired (hard stop; renewal is pointless)
func RenewLeafCert(pkiDir string, name LeafCertName) error {
	caName, ok := caForLeaf(name)
	if !ok {
		return fmt.Errorf("unknown leaf certificate %q", name)
	}

	certFile := certPath(pkiDir, string(name))
	oldCert, err := pkiutil.LoadCert(certFile)
	if err != nil {
		if isNotExistError(err) {
			return &CertMissingError{BaseName: string(name)}
		}
		return fmt.Errorf("load cert %q: %w", name, err)
	}

	caCertFile := certPath(pkiDir, string(caName))
	caCert, err := pkiutil.LoadCert(caCertFile)
	if err != nil {
		if isNotExistError(err) {
			return fmt.Errorf("CA cert %q not found", caName)
		}
		return fmt.Errorf("load CA cert %q: %w", caName, err)
	}

	if time.Now().After(caCert.NotAfter) {
		return &CAExpiredError{CAName: string(caName), ExpiredAt: caCert.NotAfter}
	}

	caKey, err := pkiutil.LoadKey(keyPath(pkiDir, string(caName)))
	if err != nil {
		if isNotExistError(err) {
			return &CAExternalError{CAName: string(caName)}
		}
		return fmt.Errorf("load CA key %q: %w", caName, err)
	}

	cfg := certConfigFromX509(oldCert)
	cfg.CertificateValidityPeriod = constants.CertificateValidityPeriod

	newKey, err := pkiutil.NewPrivateKey(cfg.EncryptionAlgorithm)
	if err != nil {
		return fmt.Errorf("generate new key for cert %q: %w", name, err)
	}

	newCert, err := pkiutil.NewSignedCert(cfg, newKey, caCert, caKey)
	if err != nil {
		return fmt.Errorf("sign cert %q: %w", name, err)
	}
	if err := writeCertAndKey(pkiDir, string(name), newCert, newKey); err != nil {
		return fmt.Errorf("write cert %q: %w", name, err)
	}
	return nil
}
