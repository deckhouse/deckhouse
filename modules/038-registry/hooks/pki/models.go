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

// Package pki is the registry module's PKI controller hook. It owns a single
// module CA plus the token/agent/distribution/auth leaf certificates and the
// local ro/rw registry users, persisting them in the registry-module-pki secret
// and exposing them under registry.internal.pki for the Helm templates that
// render the agent's secrets.
package pki

import (
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// Role values for local registry users (capital-first; matches the RegistryConfig CRD enum).
const (
	RoleReadOnly  = "ReadOnly"
	RoleReadWrite = "ReadWrite"
)

// CertModel is a PEM-encoded certificate and its private key, as stored in
// values and secrets.
type CertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

func (cm *CertModel) toPKI() (pki.CertKey, error) {
	if cm == nil {
		return pki.CertKey{}, fmt.Errorf("cannot convert nil to CertKey")
	}
	return pki.DecodeCertKey([]byte(cm.Cert), []byte(cm.Key))
}

func certModelFromPKI(value pki.CertKey) (*CertModel, error) {
	cert, key, err := pki.EncodeCertKey(value)
	if err != nil {
		return nil, fmt.Errorf("cannot encode cert/key: %w", err)
	}
	return &CertModel{Cert: string(cert), Key: string(key)}, nil
}

// secretDataToCertModel reads "<key>.crt" / "<key>.key" from secret data into a
// CertModel, returning nil when either half is missing.
func secretDataToCertModel(data map[string][]byte, key string) *CertModel {
	cert := string(data[key+".crt"])
	k := string(data[key+".key"])
	if cert == "" || k == "" {
		return nil
	}
	return &CertModel{Cert: cert, Key: k}
}

// UserModel is a local registry user: name, plaintext password (kept for
// downstream consumers such as bootstrap and the in-cluster deckhouse-registry
// secret), bcrypt hash, and role.
type UserModel struct {
	Name         string `json:"name"`
	Password     string `json:"password,omitempty"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
}

// State is the persisted PKI material (rendered to registry-module-pki and read
// back via snapshot).
type State struct {
	CA           *CertModel  `json:"ca,omitempty"`
	Token        *CertModel  `json:"token,omitempty"`
	Agent        *CertModel  `json:"agent,omitempty"`
	Distribution *CertModel  `json:"distribution,omitempty"`
	Auth         *CertModel  `json:"auth,omitempty"`
	Users        []UserModel `json:"users,omitempty"`
	HTTPSecret   string      `json:"httpSecret,omitempty"`
}

// usersFromJSON decodes a JSON-encoded []UserModel (the registry-module-pki
// "users.json" key), tolerating absent/empty input.
func usersFromJSON(raw []byte) ([]UserModel, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var users []UserModel
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, fmt.Errorf("unmarshal users: %w", err)
	}
	return users, nil
}

// Values is what the hook writes to registry.internal.pki: the full State plus a
// content hash used for secret change-tracking annotations.
type Values struct {
	State
	Hash string `json:"hash,omitempty"`
}

// Inputs carries the reuse sources for Process.
type Inputs struct {
	// FromInit is the dhctl-seeded registry-init secret (CA + ro/rw users) —
	// the highest-priority reuse source so steady-state matches bootstrap.
	// nil when absent (cluster not bootstrapped via dhctl or secret already removed).
	FromInit *State
	// FromRegistryPKI is the orchestrator's registry-pki secret (read-only),
	// used to reuse the CA/token during in-place migration. nil when absent.
	FromRegistryPKI *State
	// FromModulePKI is this hook's own persisted store (registry-module-pki) or,
	// before the secret round-trips, the prior values.State. nil when absent.
	FromModulePKI *State
}

// Result is the decoded, usable PKI material produced by Process.
type Result struct {
	CA           pki.CertKey
	Token        pki.CertKey
	Agent        pki.CertKey
	Distribution pki.CertKey
	Auth         pki.CertKey
	Users        []UserModel
	HTTPSecret   string
}
