// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tls

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

type CertKeyType string

var (
	CertKeyTypeRSA     CertKeyType = "RSA"
	CertKeyTypeED25519 CertKeyType = "ED25519"
)

func GenerateCertificate(serviceName, clusterDomain string, keyType CertKeyType, durationInDays int) (*tls.Certificate, error) {
	now := time.Now()

	subjectKeyId := make([]byte, 10)
	rand.Read(subjectKeyId)

	commonName := fmt.Sprintf("%s.%s", serviceName, clusterDomain)
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName:         commonName,
			Country:            []string{"Unknown"},
			Organization:       []string{clusterDomain},
			OrganizationalUnit: []string{serviceName},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, durationInDays),
		SubjectKeyId:          subjectKeyId,
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	var (
		priv interface{}
		pub  interface{}
		err  error
	)
	switch keyType {
	case CertKeyTypeRSA:
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA private key: %v", err)
		}
		pub = priv.(*rsa.PrivateKey).Public()
	case CertKeyTypeED25519:
		pub, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 private key: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %s (must be CertKeyRSA or CertKeyED25519)", keyType)
	}

	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  priv,
	}

	return tlsCert, nil
}
