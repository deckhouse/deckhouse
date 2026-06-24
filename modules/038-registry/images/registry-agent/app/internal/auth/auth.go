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

// Package auth provides the registry-agent's local Basic-auth authenticator,
// validating client credentials against bcrypt password hashes.
package auth

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"sigs.k8s.io/yaml"
)

// User is a local registry user with a bcrypt password hash.
type User struct {
	Name         string `json:"name"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
}

// Authenticator validates Basic credentials against bcrypt hashes.
type Authenticator struct {
	hashes map[string]string // user -> bcrypt hash
}

// New builds an Authenticator from users.
func New(users []User) *Authenticator {
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.Name] = u.PasswordHash
	}
	return &Authenticator{hashes: m}
}

// Authenticate reports whether user/password match a known bcrypt hash.
func (a *Authenticator) Authenticate(user, password string) bool {
	hash, ok := a.hashes[user]
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// LoadUsers reads a users.yaml file of the form:
//
//	users:
//	  - name: ro
//	    passwordHash: "$2a$..."
//	    role: ReadOnly
func LoadUsers(path string) ([]User, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read users file %q: %w", path, err)
	}
	var doc struct {
		Users []User `json:"users"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse users file %q: %w", path, err)
	}
	return doc.Users, nil
}
