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
	"net"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
)

func TestValidateCert_PassesValid(t *testing.T) {
	cert := makeCertForValidation(t, certutil.Config{CommonName: "test"})
	err := validateCert(cert, certConfig{
		Config:              certutil.Config{CommonName: "test"},
		EncryptionAlgorithm: constants.EncryptionAlgorithmRSA2048,
	})
	assert.NoError(t, err)
}

func TestValidateCert_FailsExpiringSoon(t *testing.T) {
	cert, key := makeExpiringSoonCACert(t, "test")
	_ = key
	err := validateCert(cert, certConfig{Config: certutil.Config{CommonName: "test"}})
	assert.Error(t, err)
}

func TestValidateCert_FailsSubjectMismatch(t *testing.T) {
	cert := makeCertForValidation(t, certutil.Config{CommonName: "original"})
	err := validateCert(cert, certConfig{Config: certutil.Config{CommonName: "different"}})
	assert.Error(t, err)
}

func TestValidateCert_FailsEncryptionAlgorithmMismatch(t *testing.T) {
	// cert with RSA-2048
	cert := makeCertForValidation(t, certutil.Config{CommonName: "test"})
	err := validateCert(cert, certConfig{
		Config:              certutil.Config{CommonName: "test"},
		EncryptionAlgorithm: constants.EncryptionAlgorithmECDSAP384,
	})
	assert.Error(t, err)
}

func TestCertificateExpiresSoon(t *testing.T) {
	expiresIn10Days := makeCertExpiringIn(t, 10*24*time.Hour)
	expiresIn60Days := makeCertExpiringIn(t, 60*24*time.Hour)

	assert.True(t, certificateExpiresSoon(expiresIn10Days, 30*24*time.Hour),
		"cert expiring in 10 days should trigger the 30-day threshold")
	assert.False(t, certificateExpiresSoon(expiresIn60Days, 30*24*time.Hour),
		"cert expiring in 60 days should not trigger the 30-day threshold")
}

func TestCertificateSubjectAndSansIsEqual(t *testing.T) {
	// Base certificate: CN=base-cn, Org=org1, DNS=example.com+foo.example.com, IP=10.0.0.1, RSA-2048
	baseCert := makeCertForValidation(t, certutil.Config{
		CommonName:   "base-cn",
		Organization: []string{"org1"},
		AltNames: certutil.AltNames{
			DNSNames: []string{"example.com", "foo.example.com"},
			IPs:      []net.IP{net.ParseIP("10.0.0.1")},
		},
	})

	tests := []struct {
		name   string
		cfg    certConfig
		wantOk bool
	}{
		{
			name: "all required fields present",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "base-cn",
				Organization: []string{"org1"},
				AltNames: certutil.AltNames{
					DNSNames: []string{"example.com"},
					IPs:      []net.IP{net.ParseIP("10.0.0.1")},
				},
			}},
			wantOk: true,
		},
		{
			name: "extra SANs in cert are allowed (subset check)",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "base-cn",
				Organization: []string{"org1"},
				// requires only example.com; cert also has foo.example.com - that is fine
				AltNames: certutil.AltNames{DNSNames: []string{"example.com"}},
			}},
			wantOk: true,
		},
		{
			name: "CN mismatch",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "other-cn",
				Organization: []string{"org1"},
			}},
			wantOk: false,
		},
		{
			name: "organization mismatch",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "base-cn",
				Organization: []string{"other-org"},
			}},
			wantOk: false,
		},
		{
			name: "required DNS SAN missing from cert",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "base-cn",
				Organization: []string{"org1"},
				AltNames:     certutil.AltNames{DNSNames: []string{"missing.example.com"}},
			}},
			wantOk: false,
		},
		{
			name: "required IP SAN missing from cert",
			cfg: certConfig{Config: certutil.Config{
				CommonName:   "base-cn",
				Organization: []string{"org1"},
				AltNames:     certutil.AltNames{IPs: []net.IP{net.ParseIP("192.168.1.1")}},
			}},
			wantOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := certificateSubjectAndSansIsEqual(baseCert, tc.cfg)
			assert.Equal(t, tc.wantOk, got)
		})
	}
}

// makeCertExpiringIn creates a certificate whose NotAfter is d from now.
func makeCertExpiringIn(t *testing.T, d time.Duration) *x509.Certificate {
	t.Helper()
	key, err := pkiutil.NewPrivateKey(constants.EncryptionAlgorithmRSA2048)
	require.NoError(t, err)

	cert, err := pkiutil.NewSelfSignedCACert(certConfig{
		Config:                    certutil.Config{CommonName: "test"},
		CertificateValidityPeriod: constants.CACertificateValidityPeriod,
		NotAfter:                  time.Now().Add(d),
	}, key)
	require.NoError(t, err)
	return cert
}
