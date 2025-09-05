// Copyright 2025 Flant JSC
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

package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateJWT(t *testing.T) {
	// Test private key (this is a test key, not for production use)
	// Ed25519 private key in PKCS8 format
	privKeyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIMgNk3rr2AmIIlkKTAM9fG6+hMKvwF+pMAT3ID3M0OFK
-----END PRIVATE KEY-----`)

	claims := map[string]string{
		"iss":   "test-issuer",
		"aud":   "test-audience",
		"sub":   "test-subject",
		"scope": "test-scope",
	}

	ttl := time.Hour

	token, err := GenerateJWT(privKeyPEM, claims, ttl)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Test that token is a valid JWT format
	assert.Contains(t, token, ".")
}

func TestIsJWTValid(t *testing.T) {
	// Test private key (this is a test key, not for production use)
	// Ed25519 private key in PKCS8 format
	privKeyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIMgNk3rr2AmIIlkKTAM9fG6+hMKvwF+pMAT3ID3M0OFK
-----END PRIVATE KEY-----`)

	claims := map[string]string{
		"iss":   "test-issuer",
		"aud":   "test-audience",
		"sub":   "test-subject",
		"scope": "test-scope",
	}

	t.Run("Valid JWT", func(t *testing.T) {
		ttl := time.Hour
		token, err := GenerateJWT(privKeyPEM, claims, ttl)
		require.NoError(t, err)

		isValid, jwtClaims, err := IsJWTValid(token)
		require.NoError(t, err)
		assert.True(t, isValid)
		assert.NotNil(t, jwtClaims)
		assert.Equal(t, "test-issuer", jwtClaims.Iss)
		assert.Equal(t, "test-audience", jwtClaims.Aud)
		assert.Equal(t, "test-subject", jwtClaims.Sub)
		assert.Equal(t, "test-scope", jwtClaims.Scope)
	})

	t.Run("Expired JWT", func(t *testing.T) {
		ttl := -time.Hour // Negative TTL makes it expired
		token, err := GenerateJWT(privKeyPEM, claims, ttl)
		require.NoError(t, err)

		isValid, jwtClaims, err := IsJWTValid(token)
		require.NoError(t, err)
		assert.False(t, isValid)
		assert.NotNil(t, jwtClaims)
		assert.True(t, jwtClaims.Exp < time.Now().Unix())
	})

	t.Run("Empty token", func(t *testing.T) {
		isValid, jwtClaims, err := IsJWTValid("")
		require.NoError(t, err)
		assert.False(t, isValid)
		assert.Nil(t, jwtClaims)
	})

	t.Run("Invalid JWT format", func(t *testing.T) {
		isValid, jwtClaims, err := IsJWTValid("invalid.jwt.token")
		require.Error(t, err)
		assert.False(t, isValid)
		assert.Nil(t, jwtClaims)
	})
}

func TestJWTClaims(t *testing.T) {
	claims := JWTClaims{
		Iss:   "test-issuer",
		Aud:   "test-audience",
		Sub:   "test-subject",
		Scope: "test-scope",
		Nbf:   time.Now().Unix(),
		Exp:   time.Now().Add(time.Hour).Unix(),
	}

	assert.Equal(t, "test-issuer", claims.Iss)
	assert.Equal(t, "test-audience", claims.Aud)
	assert.Equal(t, "test-subject", claims.Sub)
	assert.Equal(t, "test-scope", claims.Scope)
	assert.True(t, claims.Nbf > 0)
	assert.True(t, claims.Exp > claims.Nbf)
}
