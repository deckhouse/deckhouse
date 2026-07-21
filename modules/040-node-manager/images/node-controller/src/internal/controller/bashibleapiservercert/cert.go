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

package bashibleapiservercert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sort"
	"time"
)

const (
	// caExpiry and certExpiry mirror the hook: 10 years each (WithCAExpiry 87600h /
	// WithSigningDefaultExpiry 87600h). certOutdatedDuration is the 6-month renewal
	// threshold — the cert is regenerated once less than that remains.
	caExpiry             = (24 * time.Hour) * 365 * 10
	certExpiry           = (24 * time.Hour) * 365 * 10
	certOutdatedDuration = (24 * time.Hour) * 365 / 2
)

// certBundle is the self-signed CA plus the leaf serving certificate signed by it. The
// three PEM blobs map onto the Secret keys ca.crt/apiserver.crt/apiserver.key.
type certBundle struct {
	caPEM   []byte
	certPEM []byte
	keyPEM  []byte
}

// generateBundle creates a fresh self-signed CA and a leaf serving certificate for the
// given SANs, mirroring go_lib/certificate GenerateCA + GenerateSelfSignedCert (ecdsa
// P256, 10-year expiry). IP-shaped SANs go into IPAddresses and the rest into DNSNames,
// exactly as cfssl's WithSANs splits them.
//
// The leaf carries no ExtKeyUsage. The hook requested usages
// signing/key-encipherment/requestheader-client; cfssl maps the first two to KeyUsage bits
// and silently ignores the unknown "requestheader-client", so the produced cert has no EKU
// extension and is thus valid for any purpose. The kube-aggregator, which dials the
// bashible-apiserver Service as a TLS client, accepts such a serving cert.
func generateBundle(sans []string) (certBundle, error) {
	cn := certCN
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

	dnsNames, ipAddresses := splitSANs(sans)

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
		DNSNames:     dnsNames,
		IPAddresses:  ipAddresses,
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(certExpiry),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
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
// This keeps the reconcile a no-op while a good cert is in place (zero-disruption).
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
	dnsNames, ipAddresses := splitSANs(sans)
	return stringsEqual(leaf.DNSNames, dnsNames) && ipsEqual(leaf.IPAddresses, ipAddresses)
}

// splitSANs separates IP-shaped SANs from DNS SANs the same way cfssl's WithSANs does.
func splitSANs(sans []string) ([]string, []net.IP) {
	var dnsNames []string
	var ipAddresses []net.IP
	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, s)
		}
	}
	return dnsNames, ipAddresses
}

func parseCert(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func stringsEqual(a, b []string) bool {
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

func ipsEqual(a, b []net.IP) bool {
	if len(a) != len(b) {
		return false
	}
	as := make([]string, len(a))
	bs := make([]string, len(b))
	for i := range a {
		as[i] = a[i].String()
	}
	for i := range b {
		bs[i] = b[i].String()
	}
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
