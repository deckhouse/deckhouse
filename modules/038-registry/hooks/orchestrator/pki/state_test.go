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
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bootstrap"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestState_Process(t *testing.T) {
	newLogger := log.NewLogger()
	var log go_hook.Logger = newLogger
	// --- Test Case 1: Successful load ---
	t.Run("Successful load from state", func(t *testing.T) {
		ca, err := pki.GenerateCACertificate("test-ca")
		require.NoError(t, err)
		token, err := pki.GenerateCertificate("test-token", ca)
		require.NoError(t, err)

		caModel, err := pkiCertModel(ca)
		require.NoError(t, err)
		tokenModel, err := pkiCertModel(token)
		require.NoError(t, err)

		state := &State{
			CA:    caModel,
			Token: tokenModel,
		}

		result, err := state.Process(log, bootstrap.Inputs{})
		require.NoError(t, err)

		assert.True(t, ca.Cert.Equal(result.CA.Cert), "CA certificate should match")
		assert.True(t, token.Cert.Equal(result.Token.Cert), "Token certificate should match")
		assert.Equal(t, caModel, state.CA, "State CA should be unchanged")
		assert.Equal(t, tokenModel, state.Token, "State Token should be unchanged")
	})

	// --- Test Case 2: Generate new CA and Token ---
	t.Run("Generate new CA and Token when state is empty", func(t *testing.T) {
		state := &State{} // Empty state

		result, err := state.Process(log, bootstrap.Inputs{})
		require.NoError(t, err)

		assert.NotNil(t, result.CA.Cert, "Should generate a CA certificate")
		assert.NotNil(t, result.CA.Key, "Should generate a CA key")
		assert.NotNil(t, result.Token.Cert, "Should generate a Token certificate")
		assert.NotNil(t, result.Token.Key, "Should generate a Token key")

		// Check that the generated token is valid with the new CA
		err = pki.ValidateCertWithCAChain(result.Token.Cert, result.CA.Cert)
		assert.NoError(t, err, "Token should be validated by the new CA")

		// Check that state is updated
		assert.NotNil(t, state.CA)
		assert.NotEmpty(t, state.CA.Cert)
		assert.NotNil(t, state.Token)
		assert.NotEmpty(t, state.Token.Cert)
	})

	// --- Test Case 3: Token is invalid, regenerate token ---
	t.Run("Regenerate Token if it is invalid", func(t *testing.T) {
		// Create a valid CA
		ca, err := pki.GenerateCACertificate("test-ca")
		require.NoError(t, err)
		caModel, err := pkiCertModel(ca)
		require.NoError(t, err)

		// Create a self-signed token (invalid for the CA)
		invalidToken, err := pki.GenerateCACertificate("invalid-token")
		require.NoError(t, err)
		invalidTokenModel, err := pkiCertModel(invalidToken)
		require.NoError(t, err)

		state := &State{
			CA:    caModel,
			Token: invalidTokenModel,
		}

		result, err := state.Process(log, bootstrap.Inputs{})
		require.NoError(t, err)

		// CA should remain the same
		assert.True(t, ca.Cert.Equal(result.CA.Cert))
		assert.Equal(t, caModel, state.CA)

		// Token should be new
		assert.False(t, invalidToken.Cert.Equal(result.Token.Cert), "Token should be regenerated")
		err = pki.ValidateCertWithCAChain(result.Token.Cert, ca.Cert)
		assert.NoError(t, err, "New token should be valid with the CA")

		// State should be updated with the new token
		assert.NotEqual(t, invalidTokenModel, state.Token)
	})
}
