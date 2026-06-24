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

package registryaddress

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestBuildLocalDockerCfg(t *testing.T) {
	raw, err := buildLocalDockerCfg("ro", "pw")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Auths map[string]struct {
			Username, Password, Auth string
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	e, ok := cfg.Auths["registry.d8-system.svc:5001"]
	if !ok {
		t.Fatalf("missing auths entry for the local svc; got %v", cfg.Auths)
	}
	if e.Username != "ro" || e.Password != "pw" {
		t.Errorf("creds: got %q/%q", e.Username, e.Password)
	}
	if e.Auth != base64.StdEncoding.EncodeToString([]byte("ro:pw")) {
		t.Errorf("auth field: got %q", e.Auth)
	}
}

func TestBuildConstantConfig(t *testing.T) {
	c, err := buildConstantConfig("MODULE-CA", "ro", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != "registry.d8-system.svc:5001" {
		t.Errorf("address: %q", c.Address)
	}
	if c.Path != "/system/deckhouse" {
		t.Errorf("path: %q", c.Path)
	}
	if c.Scheme != "https" {
		t.Errorf("scheme: %q", c.Scheme)
	}
	if c.CA != "MODULE-CA" {
		t.Errorf("ca: %q", c.CA)
	}
	// base (what the global hook computes) must be the constant prefix.
	if c.Address+c.Path != "registry.d8-system.svc:5001/system/deckhouse" {
		t.Errorf("global base would be %q", c.Address+c.Path)
	}
	if len(c.DockerConfig) == 0 {
		t.Error("DockerConfig must be set")
	}
}
