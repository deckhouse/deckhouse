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

package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// The installer globs the path and installs whatever matches, so a path that
// matches nothing installs nothing and reports no error.
func TestRegistryUpstreamCRDPathNamesTheCRD(t *testing.T) {
	path := filepath.Join("..", strings.TrimPrefix(registryUpstreamCRDPath, "/deckhouse/"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var crd struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal(content, &crd); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if crd.Metadata.Name != "registryupstreams.deckhouse.io" {
		t.Fatalf("%s holds %q, want registryupstreams.deckhouse.io", path, crd.Metadata.Name)
	}
}
