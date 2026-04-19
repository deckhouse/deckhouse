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

package pki

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	v1core "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

func TestSecretDataToCertModel(t *testing.T) {
	tests := []struct {
		name     string
		secret   v1core.Secret
		keyName  string
		expected *CertModel
	}{
		{
			name: "Valid CA data",
			secret: v1core.Secret{
				Data: map[string][]byte{
					"ca.crt": []byte("ca-cert-data"),
					"ca.key": []byte("ca-key-data"),
				},
			},
			keyName: "ca",
			expected: &CertModel{
				Cert: "ca-cert-data",
				Key:  "ca-key-data",
			},
		},
		{
			name: "Valid token data",
			secret: v1core.Secret{
				Data: map[string][]byte{
					"token.crt": []byte("token-cert-data"),
					"token.key": []byte("token-key-data"),
				},
			},
			keyName: "token",
			expected: &CertModel{
				Cert: "token-cert-data",
				Key:  "token-key-data",
			},
		},
		{
			name: "Missing key",
			secret: v1core.Secret{
				Data: map[string][]byte{
					"ca.crt": []byte("ca-cert-data"),
				},
			},
			keyName:  "ca",
			expected: nil,
		},
		{
			name: "Missing cert",
			secret: v1core.Secret{
				Data: map[string][]byte{
					"ca.key": []byte("ca-key-data"),
				},
			},
			keyName:  "ca",
			expected: nil,
		},
		{
			name:     "Empty secret data",
			secret:   v1core.Secret{Data: map[string][]byte{}},
			keyName:  "ca",
			expected: nil,
		},
		{
			name: "Empty key name",
			secret: v1core.Secret{
				Data: map[string][]byte{
					"ca.crt": []byte("ca-cert-data"),
					"ca.key": []byte("ca-key-data"),
				},
			},
			keyName:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secretDataToCertModel(tt.secret, tt.keyName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCertModelRoundTrip(t *testing.T) {
	// 1. Generate a real CertKey object
	originalPki, err := pki.GenerateCACertificate("test-ca")
	assert.NoError(t, err)

	// 2. Convert to CertModel
	model, err := pkiCertModel(originalPki)
	assert.NoError(t, err)
	assert.NotEmpty(t, model.Cert)
	assert.NotEmpty(t, model.Key)

	// 3. Convert back to CertKey
	restoredPki, err := model.toPKI()
	assert.NoError(t, err)

	// 4. Check for equality
	assert.True(t, originalPki.Cert.Equal(restoredPki.Cert), "Certificates should be equal after round trip")

	originalKeyBytes, err := x509.MarshalPKCS8PrivateKey(originalPki.Key)
	assert.NoError(t, err)
	restoredKeyBytes, err := x509.MarshalPKCS8PrivateKey(restoredPki.Key)
	assert.NoError(t, err)
	assert.Equal(t, originalKeyBytes, restoredKeyBytes, "Keys should be equal after round trip")
}

func TestCertModelToPKI_NilReceiver(t *testing.T) {
	var model *CertModel
	_, err := model.toPKI()
	assert.Error(t, err)
	assert.Equal(t, "cannot convert nil to CertKey", err.Error())
}

func TestPkiCertModel_EncodingError(t *testing.T) {
	// Create a malformed CertKey to force an error during encoding
	ca, err := pki.GenerateCACertificate("test-ca")
	assert.NoError(t, err)

	badPki := pki.CertKey{
		Cert: ca.Cert,
		Key:  nil, // This should cause encoding to fail gracefully
	}
	_, err = pkiCertModel(badPki)
	assert.Error(t, err, "Encoding should fail when key is nil")
}

func TestToPKI_DecodingError(t *testing.T) {
	// Create a malformed CertModel to force an error during decoding
	badModel := &CertModel{
		Cert: "not-a-pem-cert",
		Key:  "not-a-pem-key",
	}
	_, err := badModel.toPKI()
	assert.Error(t, err)
}
