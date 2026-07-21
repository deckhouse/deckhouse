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

package capiwebhookcert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sort"
	"time"
)

const (
	// caExpiry and certExpiry mirror the hook: 10 years each. certOutdatedDuration is the
	// 6-month renewal threshold — a cert is regenerated once less than that remains.
	caExpiry             = (24 * time.Hour) * 365 * 10
	certExpiry           = (24 * time.Hour) * 365 * 10
	certOutdatedDuration = (24 * time.Hour) * 365 / 2
)

// certBundle is the self-signed CA plus the leaf serving certificate signed by it.
// The three PEM blobs map onto the kubernetes.io/tls Secret keys ca.crt/tls.crt/tls.key.
type certBundle struct {
	caPEM   []byte
	certPEM []byte
	keyPEM  []byte
}

// generateBundle creates a fresh self-signed CA and a leaf serving certificate for the
// given SANs, mirroring go_lib/certificate GenerateCA + GenerateSelfSignedCert (ecdsa
// P256, 10-year expiry). The leaf carries ServerAuth so the API server accepts it when it
// dials the webhook service, plus ClientAuth to match the hook's usage set.
func generateBundle(cn string, sans []string) (certBundle, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certBundle{}, fmt.Errorf("generate CA key: %w", err)
	}
	caSerial, err := randomSerial()
	if err != nil {
		return certBundle{}, err
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caExpiry),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return certBundle{}, fmt.Errorf("create CA certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return certBundle{}, fmt.Errorf("parse CA certificate: %w", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certBundle{}, fmt.Errorf("generate leaf key: %w", err)
	}
	leafSerial, err := randomSerial()
	if err != nil {
		return certBundle{}, err
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: leafSerial,
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     sans,
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(certExpiry),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return certBundle{}, fmt.Errorf("create leaf certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return certBundle{}, fmt.Errorf("marshal leaf key: %w", err)
	}

	return certBundle{
		caPEM:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}, nil
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	return serial, nil
}

// bundleValid reports whether the stored CA and leaf are still usable: both must have more
// than certOutdatedDuration left and the leaf's SANs must match the desired set exactly.
// This mirrors the hook's isOutdatedCA + isIrrelevantCert checks so the reconcile is a
// no-op while a good cert is in place (zero-disruption on rollout).
func bundleValid(caPEM, certPEM []byte, sans []string) bool {
	ca, err := parseCert(caPEM)
	if err != nil {
		return false
	}
	if time.Until(ca.NotAfter) < certOutdatedDuration {
		return false
	}
	leaf, err := parseCert(certPEM)
	if err != nil {
		return false
	}
	if time.Until(leaf.NotAfter) < certOutdatedDuration {
		return false
	}
	return sansEqual(leaf.DNSNames, sans)
}

func parseCert(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func sansEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
