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

// Package creds loads the module PKI secret and builds the mutate.Local value
// that the webhook injects into every ModuleSource.
package creds

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"registry-webhook/internal/mutate"
)

// pkiUser mirrors the shape written by the PKI hook's internal.pki.users field.
type pkiUser struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
}

// Load reads the registry-module-pki secret from dir (mounted at a known path)
// and returns a mutate.Local ready for injection. dir must contain:
//
//	ca.crt     – module CA PEM
//	users.json – JSON array of pkiUser
func Load(dir string) (mutate.Local, error) {
	caBytes, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		return mutate.Local{}, fmt.Errorf("creds: read ca.crt: %w", err)
	}

	usersBytes, err := os.ReadFile(filepath.Join(dir, "users.json"))
	if err != nil {
		return mutate.Local{}, fmt.Errorf("creds: read users.json: %w", err)
	}

	var users []pkiUser
	if err := json.Unmarshal(usersBytes, &users); err != nil {
		return mutate.Local{}, fmt.Errorf("creds: parse users.json: %w", err)
	}

	dockerCfg, err := buildDockerCfg(users, mutate.PrimarySvc)
	if err != nil {
		return mutate.Local{}, err
	}

	return mutate.Local{
		ModuleCA:  string(caBytes),
		DockerCfg: dockerCfg,
	}, nil
}

// buildDockerCfg picks the first ReadOnly user and constructs a base64-encoded
// docker config JSON for the given registry address.
func buildDockerCfg(users []pkiUser, registry string) (string, error) {
	var ro *pkiUser
	for i := range users {
		if users[i].Role == "ReadOnly" {
			ro = &users[i]
			break
		}
	}
	if ro == nil {
		return "", fmt.Errorf("creds: no ReadOnly user found in users.json")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(ro.Name + ":" + ro.Password))

	type authEntry struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}
	cfg := struct {
		Auths map[string]authEntry `json:"auths"`
	}{
		Auths: map[string]authEntry{
			registry: {
				Username: ro.Name,
				Password: ro.Password,
				Auth:     auth,
			},
		},
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("creds: marshal docker config: %w", err)
	}

	return base64.StdEncoding.EncodeToString(cfgJSON), nil
}
