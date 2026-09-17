/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
)

// testCABundle builds the base64-encoded PEM bundle the settings carry. The rule parses the
// certificate and insists it is a CA, so the fixture has to be a real one — generating it keeps it
// valid by construction rather than by a pasted blob that expires.
func testCABundle(t *testing.T) string {
	t.Helper()

	return testCertificateBundle(t, true)
}

// testLeafCertificateBundle is a well-formed certificate that is not a CA.
func testLeafCertificateBundle(t *testing.T) string {
	t.Helper()

	return testCertificateBundle(t, false)
}

func testCertificateBundle(t *testing.T, isCA bool) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "zvirt-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// testPEMBlock encodes an arbitrary PEM block, used to check that a non-certificate block is
// rejected even though it is perfectly well-formed PEM.
func testPEMBlock(blockType string) string {
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: []byte("payload")}))
}

func hasCode(violations []cpvalapi.Violation, code string) bool {
	for _, violation := range violations {
		if violation.Code == code {
			return true
		}
	}

	return false
}

func TestValidateProviderConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		server      string
		caBundle    func(*testing.T) string
		insecure    bool
		wantError   string
		wantWarning string
	}{
		{
			name:     "a plain https endpoint is accepted",
			server:   "https://zvirt.example.com/ovirt-engine/api",
			caBundle: testCABundle,
		},
		{
			name:      "an empty endpoint is rejected",
			server:    "   ",
			wantError: CodeProviderServerRequired,
		},
		{
			name:      "an endpoint without a scheme is rejected",
			server:    "zvirt.example.com/ovirt-engine/api",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "an endpoint with a foreign scheme is rejected",
			server:    "ftp://zvirt.example.com",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "an endpoint without a host is rejected",
			server:    "https:///ovirt-engine/api",
			wantError: CodeProviderServerInvalid,
		},
		{
			name:      "a CA bundle that is not base64 is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return "%%% not base64 %%%" },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a base64 CA bundle that is not PEM is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return base64.StdEncoding.EncodeToString([]byte("just some text")) },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a PEM block of the wrong type is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  func(*testing.T) string { return testPEMBlock("RSA PRIVATE KEY") },
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:      "a well-formed certificate that is not a CA is rejected",
			server:    "https://zvirt.example.com",
			caBundle:  testLeafCertificateBundle,
			wantError: CodeProviderCABundleInvalid,
		},
		{
			name:        "a CA bundle together with insecure only warns",
			server:      "https://zvirt.example.com",
			caBundle:    testCABundle,
			insecure:    true,
			wantWarning: CodeProviderCABundleIgnored,
		},
		{
			name:     "insecure without a CA bundle is silent",
			server:   "https://zvirt.example.com",
			insecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			caBundle := ""
			if tt.caBundle != nil {
				caBundle = tt.caBundle(t)
			}

			state := &State{
				ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{
					Spec: cpapi.ModuleConfigSpec[*zsettingsv2.ModuleConfigSettings]{
						Settings: &zsettingsv2.ModuleConfigSettings{
							Provider: zsettingsv2.Provider{
								Parameters: zsettingsv2.ProviderParameters{
									Server:   tt.server,
									CABundle: caBundle,
									Insecure: tt.insecure,
								},
							},
						},
					},
				},
			}

			result := ValidateProviderConnection(state)
			assertResult(t, result, tt.wantError, tt.wantWarning)

			// The legacy configuration must be held to exactly the same standard: the whole point
			// of sharing the rule is that a cluster cannot pass one surface and fail the other.
			legacy := ValidateLegacyProviderConnection(&zpccv1.ZvirtProviderClusterConfiguration{
				Provider: zpccv1.ZvirtProvider{
					Server:   tt.server,
					CABundle: caBundle,
					Insecure: tt.insecure,
				},
			})
			assertResult(t, legacy, tt.wantError, tt.wantWarning)
		})
	}
}

func assertResult(t *testing.T, result cpvalapi.Result, wantError, wantWarning string) {
	t.Helper()

	switch {
	case wantError == "" && result.HasErrors():
		t.Errorf("unexpected errors: %v", result.Error())
	case wantError != "" && !hasCode(result.Errors(), wantError):
		t.Errorf("errors = %v, want code %s", result.Error(), wantError)
	}

	if wantWarning == "" {
		if len(result.Warnings()) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings())
		}

		return
	}

	if !hasCode(result.Warnings(), wantWarning) {
		t.Errorf("warnings = %v, want code %s", result.Warnings(), wantWarning)
	}
}

// A nil state reaches the rule whenever the ModuleConfig has not been created yet. That is not a
// violation on its own — the ModuleConfig rule reports it — so the rule must stay silent.
func TestValidateProviderConnectionTolerantOfMissingConfig(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]*State{
		"nil state":         nil,
		"no ModuleConfig":   {},
		"no settings":       {ModuleConfig: &cpapi.ModuleConfig[*zsettingsv2.ModuleConfigSettings]{}},
		"nil legacy config": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if result := ValidateProviderConnection(state); result.HasErrors() || len(result.Warnings()) != 0 {
				t.Errorf("ValidateProviderConnection() = %v, want silence", result.Error())
			}
		})
	}

	if result := ValidateLegacyProviderConnection(nil); result.HasErrors() {
		t.Errorf("ValidateLegacyProviderConnection(nil) = %v, want silence", result.Error())
	}
}
