// Copyright 2026 Flant JSC
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

package pki

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"golang.org/x/crypto/bcrypt"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// Common names and SANs for the module PKI.
const (
	caCN           = "registry-ca"
	tokenCN        = "registry-auth-token"
	agentCN        = "registry-agent"
	distributionCN = "registry-distribution"
	authCN         = "registry-auth"

	registryServiceDNS    = "registry.d8-system.svc"
	cacheServiceDNS       = "registry-cache.d8-system.svc"
	cacheLeaderServiceDNS = "registry-cache-leader.d8-system.svc"
)

var (
	agentHosts = []string{"127.0.0.1", "localhost", registryServiceDNS}
	// The distribution serves on both the all-replicas Service (reads) and the
	// leader Service (writes — store-sync from the bootstrap seed and the legacy
	// migration both push to registry-cache-leader), so the cert must cover both.
	distributionHosts = []string{cacheServiceDNS, cacheLeaderServiceDNS, "127.0.0.1", "localhost"}
	authHosts         = []string{"127.0.0.1", "localhost"}
)

// userSpec defines a managed local user.
type userSpec struct {
	name string
	role string
}

var managedUsers = []userSpec{
	{name: "ro", role: RoleReadOnly},
	{name: "rw", role: RoleReadWrite},
}

// Process reuses or generates the module CA, the token/agent/distribution/auth
// leaf certs, and the ro/rw users, writing the persisted form into the receiver
// and returning the decoded material.
func (state *State) Process(log go_hook.Logger, inputs Inputs) (Result, error) {
	var (
		ret Result
		err error
	)

	var initCA *CertModel
	var initUsers []UserModel
	if inputs.FromInit != nil {
		initCA = inputs.FromInit.CA
		initUsers = inputs.FromInit.Users
	}
	var regCA, regToken *CertModel
	if inputs.FromRegistryPKI != nil {
		regCA, regToken = inputs.FromRegistryPKI.CA, inputs.FromRegistryPKI.Token
	}
	var modCA, modToken, modAgent, modDist, modAuth *CertModel
	var modUsers []UserModel
	if inputs.FromModulePKI != nil {
		modCA = inputs.FromModulePKI.CA
		modToken = inputs.FromModulePKI.Token
		modAgent = inputs.FromModulePKI.Agent
		modDist = inputs.FromModulePKI.Distribution
		modAuth = inputs.FromModulePKI.Auth
		modUsers = inputs.FromModulePKI.Users
	}

	// CA: registry-init (bootstrap) -> registry-pki (migration) -> module-pki -> generate.
	ret.CA, err = reuseCA(initCA, regCA, modCA)
	if err != nil {
		log.Warn("cannot reuse module CA, generating a new one", "error", err)
		if ret.CA, err = pki.GenerateCACertificate(caCN); err != nil {
			return ret, fmt.Errorf("cannot generate CA: %w", err)
		}
	}

	// Token (signed by CA): reuse registry-pki -> module-pki -> generate.
	if ret.Token, err = reuseOrGenerate(tokenCN, ret.CA, nil, regToken, modToken); err != nil {
		return ret, fmt.Errorf("cannot obtain token cert: %w", err)
	}

	// Agent / distribution / auth leaf certs persist only in module-pki.
	if ret.Agent, err = reuseOrGenerate(agentCN, ret.CA, agentHosts, modAgent); err != nil {
		return ret, fmt.Errorf("cannot obtain agent cert: %w", err)
	}
	if ret.Distribution, err = reuseOrGenerate(distributionCN, ret.CA, distributionHosts, modDist); err != nil {
		return ret, fmt.Errorf("cannot obtain distribution cert: %w", err)
	}
	if ret.Auth, err = reuseOrGenerate(authCN, ret.CA, authHosts, modAuth); err != nil {
		return ret, fmt.Errorf("cannot obtain auth cert: %w", err)
	}

	// Local users: init wins (bootstrap seeded) -> module-pki -> generate.
	if ret.Users, err = processUsers(initUsers, modUsers); err != nil {
		return ret, fmt.Errorf("cannot obtain users: %w", err)
	}

	// HTTP secret: reuse from module-pki when present, else generate. Must be
	// stable so HA cache replicas share one value behind a single Service.
	var modHTTPSecret string
	if inputs.FromModulePKI != nil {
		modHTTPSecret = inputs.FromModulePKI.HTTPSecret
	}
	if ret.HTTPSecret = modHTTPSecret; ret.HTTPSecret == "" {
		if ret.HTTPSecret, err = pki.GenerateRandomSecret(); err != nil {
			return ret, fmt.Errorf("cannot generate HTTP secret: %w", err)
		}
	}

	if err = state.store(ret); err != nil {
		return ret, fmt.Errorf("cannot persist PKI state: %w", err)
	}
	return ret, nil
}

// reuseCA returns the first decodable CA among the candidates, or an error when
// none decode (caller generates a fresh CA).
func reuseCA(candidates ...*CertModel) (pki.CertKey, error) {
	for _, c := range candidates {
		if c == nil {
			continue
		}
		if ck, err := c.toPKI(); err == nil {
			return ck, nil
		}
	}
	return pki.CertKey{}, fmt.Errorf("no reusable CA found")
}

// reuseOrGenerate returns the first candidate cert that decodes and validates
// against ca; otherwise it generates a new leaf (cn + hosts) signed by ca.
func reuseOrGenerate(cn string, ca pki.CertKey, hosts []string, candidates ...*CertModel) (pki.CertKey, error) {
	for _, c := range candidates {
		if c == nil {
			continue
		}
		ck, err := c.toPKI()
		if err != nil {
			continue
		}
		if err := pki.ValidateCertWithCAChain(ck.Cert, ca.Cert); err != nil {
			continue
		}
		return ck, nil
	}
	return pki.GenerateCertificate(cn, ca, hosts...)
}

// processUsers resolves managed users by merging init (highest priority) and
// module-pki (persistent store) sources: init wins for any user it provides,
// module-pki fills in the rest, and any remaining managed users are generated.
func processUsers(initUsers, modUsers []UserModel) ([]UserModel, error) {
	prev := make(map[string]UserModel, len(modUsers)+len(initUsers))
	for _, u := range modUsers {
		prev[u.Name] = u
	}
	for _, u := range initUsers { // init wins over module-pki
		prev[u.Name] = u
	}

	ret := make([]UserModel, 0, len(managedUsers))
	for _, spec := range managedUsers {
		u, err := processUser(spec, prev[spec.name])
		if err != nil {
			return nil, err
		}
		ret = append(ret, u)
	}
	return ret, nil
}

func processUser(spec userSpec, prev UserModel) (UserModel, error) {
	u := UserModel{
		Name:         spec.name,
		Role:         spec.role,
		Password:     prev.Password,
		PasswordHash: prev.PasswordHash,
	}

	if u.Password == "" {
		pw, err := pki.GenerateUserPassword()
		if err != nil {
			return u, fmt.Errorf("generate password for %q: %w", spec.name, err)
		}
		u.Password = pw
		u.PasswordHash = ""
	}

	if !passwordHashValid(u.Password, u.PasswordHash) {
		hash, err := pki.GeneratePasswordHash(u.Password)
		if err != nil {
			return u, fmt.Errorf("hash password for %q: %w", spec.name, err)
		}
		u.PasswordHash = hash
	}

	return u, nil
}

func passwordHashValid(password, hash string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// store encodes the result back into the receiver for persistence.
func (state *State) store(ret Result) error {
	var err error
	if state.CA, err = certModelFromPKI(ret.CA); err != nil {
		return fmt.Errorf("encode CA: %w", err)
	}
	if state.Token, err = certModelFromPKI(ret.Token); err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	if state.Agent, err = certModelFromPKI(ret.Agent); err != nil {
		return fmt.Errorf("encode agent: %w", err)
	}
	if state.Distribution, err = certModelFromPKI(ret.Distribution); err != nil {
		return fmt.Errorf("encode distribution: %w", err)
	}
	if state.Auth, err = certModelFromPKI(ret.Auth); err != nil {
		return fmt.Errorf("encode auth: %w", err)
	}
	state.Users = ret.Users
	state.HTTPSecret = ret.HTTPSecret
	return nil
}
