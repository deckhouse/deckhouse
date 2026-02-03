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

package constants

import "time"

const (
	DefaultCertificatesDir = "/etc/kubernetes/pki"

	DefaultControlPlaneIP = "127.0.0.1"

	// CACertAndKeyBaseName defines certificate authority base name
	CACertAndKeyBaseName = "ca"

	// CertificateValidityPeriod defines the validity period for all leaf (non-CA) certificates.
	CertificateValidityPeriod = time.Hour * 24 * 365

	// CACertificateValidityPeriod defines the validity for all CA certificates.
	CACertificateValidityPeriod = time.Hour * 24 * 365 * 10

	CertificateBackdate = 5 * time.Minute

	DiscoveredNodeIPPath = "/var/lib/bashible/discovered-node-ip"

	DiscoveredNodeNamePath = "/var/lib/bashible/discovered-node-name"
)

// EncryptionAlgorithmType can define an asymmetric encryption algorithm type.
type EncryptionAlgorithmType string

const (
	// EncryptionAlgorithmECDSAP256 defines the ECDSA encryption algorithm type with curve P256.
	EncryptionAlgorithmECDSAP256 EncryptionAlgorithmType = "ECDSA-P256"
	// EncryptionAlgorithmECDSAP384 defines the ECDSA encryption algorithm type with curve P384.
	EncryptionAlgorithmECDSAP384 EncryptionAlgorithmType = "ECDSA-P384"
	// EncryptionAlgorithmRSA2048 defines the RSA encryption algorithm type with key size 2048 bits.
	EncryptionAlgorithmRSA2048 EncryptionAlgorithmType = "RSA-2048"
	// EncryptionAlgorithmRSA3072 defines the RSA encryption algorithm type with key size 3072 bits.
	EncryptionAlgorithmRSA3072 EncryptionAlgorithmType = "RSA-3072"
	// EncryptionAlgorithmRSA4096 defines the RSA encryption algorithm type with key size 4096 bits.
	EncryptionAlgorithmRSA4096 EncryptionAlgorithmType = "RSA-4096"
)

const (
	ControlPlaneLabelKey = "node-role.kubernetes.io/control-plane"
	ControlPlaneTaintKey = "node-role.kubernetes.io/control-plane"
)
