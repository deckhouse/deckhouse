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

package creds

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDockerCfg_ReadOnlyUser(t *testing.T) {
	users := []pkiUser{
		{Name: "admin", Password: "adminpass", Role: "Admin"},
		{Name: "reader", Password: "readerpass", Role: "ReadOnly"},
	}

	b64, err := buildDockerCfg(users, "registry.d8-system.svc:5001")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("result is not valid base64: %v", err)
	}

	var cfg struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Auth     string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("result is not valid docker config JSON: %v", err)
	}

	entry, ok := cfg.Auths["registry.d8-system.svc:5001"]
	if !ok {
		t.Fatal("expected auths entry for registry.d8-system.svc:5001")
	}
	if entry.Username != "reader" {
		t.Errorf("expected ReadOnly user 'reader', got %q", entry.Username)
	}
	if entry.Password != "readerpass" {
		t.Errorf("expected password 'readerpass', got %q", entry.Password)
	}

	expectedAuth := base64.StdEncoding.EncodeToString([]byte("reader:readerpass"))
	if entry.Auth != expectedAuth {
		t.Errorf("auth mismatch: got %q, want %q", entry.Auth, expectedAuth)
	}
}

func TestBuildDockerCfg_NoReadOnlyUser(t *testing.T) {
	users := []pkiUser{
		{Name: "admin", Password: "adminpass", Role: "Admin"},
	}

	_, err := buildDockerCfg(users, "registry.d8-system.svc:5001")
	if err == nil {
		t.Fatal("expected error when no ReadOnly user exists")
	}
	if !strings.Contains(err.Error(), "no ReadOnly user") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildDockerCfg_FirstReadOnlyWins(t *testing.T) {
	users := []pkiUser{
		{Name: "ro1", Password: "pass1", Role: "ReadOnly"},
		{Name: "ro2", Password: "pass2", Role: "ReadOnly"},
	}

	b64, err := buildDockerCfg(users, "registry.d8-system.svc:5001")
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := base64.StdEncoding.DecodeString(b64)
	var cfg struct {
		Auths map[string]struct {
			Username string `json:"username"`
		} `json:"auths"`
	}
	_ = json.Unmarshal(raw, &cfg)
	if cfg.Auths["registry.d8-system.svc:5001"].Username != "ro1" {
		t.Errorf("expected first ReadOnly user 'ro1'")
	}
}
