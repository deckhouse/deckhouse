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
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func testLogger() go_hook.Logger {
	return log.NewLogger()
}

func mustCertModel(t *testing.T, ck pki.CertKey) *CertModel {
	t.Helper()
	m, err := certModelFromPKI(ck)
	require.NoError(t, err)
	return m
}

func TestProcess_GenerateFresh(t *testing.T) {
	var state State
	res, err := state.Process(testLogger(), Inputs{})
	require.NoError(t, err)

	// Every cert generated and self-consistent.
	require.NotNil(t, res.CA.Cert)
	require.NotNil(t, res.Token.Cert)
	require.NotNil(t, res.Agent.Cert)
	require.NotNil(t, res.Distribution.Cert)
	require.NotNil(t, res.Auth.Cert)

	// Leaf certs chain to the generated CA.
	require.NoError(t, pki.ValidateCertWithCAChain(res.Token.Cert, res.CA.Cert))
	require.NoError(t, pki.ValidateCertWithCAChain(res.Agent.Cert, res.CA.Cert))
	require.NoError(t, pki.ValidateCertWithCAChain(res.Distribution.Cert, res.CA.Cert))
	require.NoError(t, pki.ValidateCertWithCAChain(res.Auth.Cert, res.CA.Cert))

	// SANs.
	assert.Contains(t, res.Agent.Cert.DNSNames, "registry.d8-system.svc")
	assert.Contains(t, res.Distribution.Cert.DNSNames, "registry-cache.d8-system.svc")

	// ro + rw users with bcrypt hashes.
	require.Len(t, res.Users, 2)
	byName := map[string]UserModel{}
	for _, u := range res.Users {
		byName[u.Name] = u
	}
	require.Contains(t, byName, "ro")
	require.Contains(t, byName, "rw")
	assert.Equal(t, RoleReadOnly, byName["ro"].Role)
	assert.Equal(t, RoleReadWrite, byName["rw"].Role)
	assert.NotEmpty(t, byName["ro"].Password)
	assert.True(t, passwordHashValid(byName["ro"].Password, byName["ro"].PasswordHash))

	// httpSecret generated.
	assert.NotEmpty(t, res.HTTPSecret)
	assert.NotEmpty(t, state.HTTPSecret)

	// State persisted for the next reconcile.
	assert.NotNil(t, state.CA)
	assert.NotNil(t, state.Agent)
	assert.Len(t, state.Users, 2)
}

func TestProcess_ReuseCAFromRegistryPKI(t *testing.T) {
	ca, err := pki.GenerateCACertificate("registry-ca")
	require.NoError(t, err)
	token, err := pki.GenerateCertificate("registry-auth-token", ca)
	require.NoError(t, err)

	reg := &State{CA: mustCertModel(t, ca), Token: mustCertModel(t, token)}

	var state State
	res, err := state.Process(testLogger(), Inputs{FromRegistryPKI: reg})
	require.NoError(t, err)

	// CA and token reused verbatim from registry-pki.
	assert.True(t, ca.Cert.Equal(res.CA.Cert), "CA must be reused from registry-pki")
	assert.True(t, token.Cert.Equal(res.Token.Cert), "token must be reused from registry-pki")

	// New leaf certs chain to the reused CA.
	require.NoError(t, pki.ValidateCertWithCAChain(res.Agent.Cert, res.CA.Cert))
}

func TestProcess_ReuseFromModulePKI(t *testing.T) {
	// First run generates everything.
	var first State
	res1, err := first.Process(testLogger(), Inputs{})
	require.NoError(t, err)

	// Second run sees the persisted module store; must reuse, not regenerate.
	var second State
	res2, err := second.Process(testLogger(), Inputs{FromModulePKI: &first})
	require.NoError(t, err)

	assert.True(t, res1.CA.Cert.Equal(res2.CA.Cert), "CA stable across reconciles")
	assert.True(t, res1.Agent.Cert.Equal(res2.Agent.Cert), "agent cert stable")
	assert.True(t, res1.Distribution.Cert.Equal(res2.Distribution.Cert), "distribution cert stable")
	require.Len(t, res2.Users, 2)
	// Passwords/hashes stable.
	assert.Equal(t, res1.Users[0].Password, res2.Users[0].Password)
	assert.Equal(t, res1.Users[0].PasswordHash, res2.Users[0].PasswordHash)
	assert.Equal(t, res1.HTTPSecret, res2.HTTPSecret, "httpSecret stable across reconciles")
	assert.NotEmpty(t, res2.HTTPSecret)
}

func TestProcess_ReuseCAFromInitSecret(t *testing.T) {
	ca, err := pki.GenerateCACertificate("registry-ca")
	require.NoError(t, err)

	roHash, err := pki.GeneratePasswordHash("rp")
	require.NoError(t, err)
	rwHash, err := pki.GeneratePasswordHash("wp")
	require.NoError(t, err)

	initState := &State{
		CA: mustCertModel(t, ca),
		Users: []UserModel{
			{Name: "ro", Password: "rp", PasswordHash: roHash, Role: RoleReadOnly},
			{Name: "rw", Password: "wp", PasswordHash: rwHash, Role: RoleReadWrite},
		},
	}
	res, err := (&State{}).Process(testLogger(), Inputs{FromInit: initState})
	require.NoError(t, err)
	assert.True(t, ca.Cert.Equal(res.CA.Cert), "CA must be reused from registry-init")
	byName := map[string]UserModel{}
	for _, u := range res.Users {
		byName[u.Name] = u
	}
	assert.Equal(t, "rp", byName["ro"].Password, "ro password reused from init")
	assert.Equal(t, roHash, byName["ro"].PasswordHash, "ro hash reused from init")
}

func TestProcess_InitCAWinsOverModulePKI(t *testing.T) {
	initCA, err := pki.GenerateCACertificate("registry-ca")
	require.NoError(t, err)
	otherCA, err := pki.GenerateCACertificate("other")
	require.NoError(t, err)
	res, err := (&State{}).Process(testLogger(), Inputs{
		FromInit:      &State{CA: mustCertModel(t, initCA)},
		FromModulePKI: &State{CA: mustCertModel(t, otherCA)},
	})
	require.NoError(t, err)
	assert.True(t, initCA.Cert.Equal(res.CA.Cert), "registry-init CA must win over module-pki")
}

func TestProcess_RegeneratesLeafOnCAMismatch(t *testing.T) {
	// Module store has a leaf signed by a DIFFERENT CA than the one in use.
	otherCA, err := pki.GenerateCACertificate("other-ca")
	require.NoError(t, err)
	staleAgent, err := pki.GenerateCertificate("registry-agent", otherCA, "127.0.0.1")
	require.NoError(t, err)

	currentCA, err := pki.GenerateCACertificate("registry-ca")
	require.NoError(t, err)

	mod := &State{
		CA:    mustCertModel(t, currentCA),
		Agent: mustCertModel(t, staleAgent),
	}

	var state State
	res, err := state.Process(testLogger(), Inputs{FromModulePKI: mod})
	require.NoError(t, err)

	// Agent cert regenerated to chain to currentCA (stale one rejected).
	assert.False(t, staleAgent.Cert.Equal(res.Agent.Cert), "stale agent cert must be replaced")
	require.NoError(t, pki.ValidateCertWithCAChain(res.Agent.Cert, res.CA.Cert))
}
