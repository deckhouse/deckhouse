/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestLocalCRLSignersIncludesRootAndIntermediate(t *testing.T) {
	rootCert, rootKey := mustTestCACert(t, "root", nil, nil)
	intCert, intKey := mustTestCACert(t, "int", rootCert, rootKey)

	data := map[string][]byte{
		"ca-cert.pem":   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intCert.Raw}),
		"root-cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootCert.Raw}),
	}
	signers := localCRLSigners(data)
	if len(signers) != 2 {
		t.Fatalf("expected 2 signers, got %d", len(signers))
	}
	_ = intKey
}

func TestFilterLocalCRLPEMKeepsLocalRootAndIntStripsPeer(t *testing.T) {
	rootCert, rootKey := mustTestCACert(t, "root-a", nil, nil)
	intCert, intKey := mustTestCACert(t, "int-a", rootCert, rootKey)
	peerRoot, peerRootKey := mustTestCACert(t, "root-b", nil, nil)
	peerInt, peerIntKey := mustTestCACert(t, "int-b", peerRoot, peerRootKey)

	localIntCRL := mustEmptyCRL(t, intCert, intKey, 1)
	localRootCRL := mustEmptyCRL(t, rootCert, rootKey, 1)
	peerIntCRL := mustEmptyCRL(t, peerInt, peerIntKey, 1)
	peerRootCRL := mustEmptyCRL(t, peerRoot, peerRootKey, 1)

	combined := strings.Join([]string{localIntCRL, localRootCRL, peerIntCRL, peerRootCRL}, "\n")
	signers := localCRLSigners(map[string][]byte{
		"ca-cert.pem":   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intCert.Raw}),
		"root-cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootCert.Raw}),
	})

	got := filterLocalCRLPEM(combined, signers)
	if strings.Count(got, "BEGIN X509 CRL") != 2 {
		t.Fatalf("expected 2 local CRL blocks, got %d in:\n%s", strings.Count(got, "BEGIN X509 CRL"), got)
	}
	if !strings.Contains(got, strings.TrimSpace(localIntCRL)) {
		t.Fatal("missing local intermediate CRL")
	}
	if !strings.Contains(got, strings.TrimSpace(localRootCRL)) {
		t.Fatal("missing local root CRL")
	}
	if strings.Contains(got, strings.TrimSpace(peerIntCRL)) || strings.Contains(got, strings.TrimSpace(peerRootCRL)) {
		t.Fatal("peer CRLs must be stripped from exported metadata")
	}
}

func mustTestCACert(t *testing.T, cn string, parent *x509.Certificate, parentKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"d8-test"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	signerCert := tmpl
	signerKey := key
	if parent != nil {
		signerCert = parent
		signerKey = parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return cert, key
}

func mustEmptyCRL(t *testing.T, issuer *x509.Certificate, key *rsa.PrivateKey, n int64) string {
	t.Helper()
	now := time.Now()
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(n),
		ThisUpdate: now.Add(-time.Minute),
		NextUpdate: now.Add(24 * time.Hour),
	}, issuer, key)
	if err != nil {
		t.Fatalf("create CRL: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der}))
}
