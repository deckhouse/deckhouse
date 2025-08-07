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

package verifier

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

const testDir = "testdata"

var (
	shouldGenerateCA  = false
	shouldSignModules = false
)

func Test(t *testing.T) {
	if shouldGenerateCA {
		generateCA(t)
	}

	if shouldSignModules {
		signModules(t, "validmodule", "invalidmodule")
	}

	cases := []testCase{
		{
			"validmodule",
			true,
		},
		{
			"invalidmodule",
			false,
		},
	}

	verifyModules(t, cases...)
}

type testCase struct {
	module string
	valid  bool
}

func verifyModules(t *testing.T, cases ...testCase) {
	ca, _, err := parseCA()
	if err != nil {
		t.Fatalf("failed to parse CA: %v", err)
	}

	verifier := New(ca)

	for _, test := range cases {
		def := &moduletypes.Definition{
			Name: test.module,
			Path: filepath.Join(testDir, test.module),
		}

		if err = verifier.VerifyModule(def); err != nil {
			if test.valid {
				t.Fatalf("failed to verify module %q: %v", test.module, err)
			}

			return
		}

		if !test.valid {
			t.Fatalf("the '%s' module should not be valid", test.module)
		}
	}
}

func signModules(t *testing.T, modules ...string) {
	ca, key, err := parseCA()
	if err != nil {
		t.Fatalf("failed to parse CA: %v", err)
	}

	sign := newSigner(ca, key)

	for _, module := range modules {
		if err = sign.Sign(module, filepath.Join(testDir, module)); err != nil {
			t.Fatalf("failed to sign module %s: %v", module, err)
		}
	}
}

func parseCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	// Read and decode the CA certificate PEM
	certPEM, err := os.ReadFile(filepath.Join(testDir, "ca.crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("read crt: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca crt: %w", err)
	}

	// Read and decode the CA private key PEM
	keyPEM, err := os.ReadFile(filepath.Join(testDir, "key.pem"))
	if err != nil {
		return nil, nil, fmt.Errorf("read key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		return nil, nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	priv, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca key: %w", err)
	}

	return cert, priv, nil
}

func generateCA(t *testing.T) {
	// Generate ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Deckhouse"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Encode private key to PEM
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	if err = os.WriteFile(filepath.Join(testDir, "ca.crt"), certPEM, 0644); err != nil {
		t.Fatalf("failed to write certificate: %v", err)
	}

	if err = os.WriteFile(filepath.Join(testDir, "key.pem"), keyPEM, 0600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}
}
