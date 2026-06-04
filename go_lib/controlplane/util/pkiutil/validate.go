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

package pkiutil

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
)

// CertificateExpiresSoon returns true if cert will expire within the given threshold duration.
func CertificateExpiresSoon(cert *x509.Certificate, threshold time.Duration) bool {
	return time.Until(cert.NotAfter) < threshold
}

// DetectEncryptionAlgorithm returns the EncryptionAlgorithmType corresponding to the public key of cert.
// Returns "" for unknown key types or sizes.
func DetectEncryptionAlgorithm(cert *x509.Certificate) constants.EncryptionAlgorithmType {
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
