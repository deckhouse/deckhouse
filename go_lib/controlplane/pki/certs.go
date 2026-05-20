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
	"crypto"
	"crypto/x509"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

// createCertTree creates all CA and leaf certificates defined by cfg.CertTreeScheme.
// For each CA the in-memory cert and key are passed directly to the leaf cert functions,
// so CAs are created before their leaves within each iteration.
func createCertTree(cfg config, rep *PKIApplyReport) error {
	certSpecTree := renderCertSpecTree(cfg.CertTreeScheme)

	for _, rootCertSpec := range certSpecTree {
		caCert, caKey, err := createRootCertIfNotExists(cfg, rootCertSpec, rep)
		if err != nil {
			return fmt.Errorf("failed to create root certificate %q: %w", rootCertSpec.BaseName, err)
		}

		for _, certSpec := range rootCertSpec.leafCerts {
			if err := createLeafCertIfNotExists(cfg, certSpec, caCert, caKey, rep); err != nil {
				return fmt.Errorf("failed to create certificate %q: %w", certSpec.BaseName, err)
			}
		}
	}

	return nil
}

// createRootCertIfNotExists loads an existing CA from disk and validates it, or creates a new one.
//
// Intentional asymmetry with createLeafCertIfNotExists:
// if the existing CA fails validation, a CertValidationError is returned and the process stops.
// CA certificates are never silently regenerated because doing so would invalidate all leaf
// certificates signed by that CA, requiring a full cluster PKI rotation.
func createRootCertIfNotExists(cfg config, spec rootCertSpec, rep *PKIApplyReport) (*x509.Certificate, crypto.Signer, error) {
	oldCert, oldKey, err := readCertAndKey(cfg.pkiDir, spec.BaseName)
	newCertCfg := spec.BuildConfig(cfg)
	if err == nil {
		if err := validateCert(oldCert, newCertCfg); err != nil {
			return nil, nil, &CertValidationError{
				BaseName: spec.BaseName,
				Reason:   err.Error(),
			}
		}
		rep.add(spec.BaseName, PKIEntryKindRootCA, PKIActionUnchanged)
		return oldCert, oldKey, nil
	}

	if !isNotExistError(err) {
		return nil, nil, fmt.Errorf("failed to load CA %q: %w", spec.BaseName, err)
	}

	newKey, err := pkiutil.NewPrivateKey(cfg.EncryptionAlgorithmType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate new key for CA %q: %w", spec.BaseName, err)
	}

	newCert, err := pkiutil.NewSelfSignedCACert(newCertCfg, newKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA %q: %w", spec.BaseName, err)
	}

	if err := writeCertAndKey(cfg.pkiDir, spec.BaseName, newCert, newKey); err != nil {
		return nil, nil, fmt.Errorf("failed to write CA %q: %w", spec.BaseName, err)
	}

	rep.add(spec.BaseName, PKIEntryKindRootCA, PKIActionWrittenCreated)
	return newCert, newKey, nil
}

// createLeafCertIfNotExists creates or regenerates a leaf certificate.
//
// If the certificate exists and passes validation, it is kept unchanged.
// If it exists but fails validation (expired, SANs changed, etc.), it is silently regenerated.
// If the file is absent, a new certificate is created.
//
// A read error other than "file not found" is currently treated the same as a missing file
// (the certificate is regenerated). This is intentional: a corrupted cert file should not
// block PKI initialization.
func createLeafCertIfNotExists(cfg config, spec certSpec[LeafCertName], caCert *x509.Certificate, caKey crypto.Signer, rep *PKIApplyReport) error {
	oldCert, _, err := readCertAndKey(cfg.pkiDir, spec.BaseName)
	newCertCfg := spec.BuildConfig(cfg)
	regenerate := false
	if err == nil {
		if err := validateCert(oldCert, newCertCfg); err == nil {
			rep.add(spec.BaseName, PKIEntryKindLeafCert, PKIActionUnchanged)
			return nil
		}
		regenerate = true
	} else if !isNotExistError(err) {
		log.Warn("Cert, will be recreated", slog.String("baseName", spec.BaseName), slog.String("reason", err.Error()))
		regenerate = true
	}

	newKey, err := pkiutil.NewPrivateKey(cfg.EncryptionAlgorithmType)
	if err != nil {
		return fmt.Errorf("failed to generate new key for cert %q: %w", spec.BaseName, err)
	}

	newCert, err := pkiutil.NewSignedCert(newCertCfg, newKey, caCert, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate cert %q: %w", spec.BaseName, err)
	}

	if err := writeCertAndKey(cfg.pkiDir, spec.BaseName, newCert, newKey); err != nil {
		return fmt.Errorf("failed to write cert %q: %w", spec.BaseName, err)
	}

	if regenerate {
		rep.add(spec.BaseName, PKIEntryKindLeafCert, PKIActionWrittenRegenerated)
	} else {
		rep.add(spec.BaseName, PKIEntryKindLeafCert, PKIActionWrittenCreated)
	}
	return nil
}
