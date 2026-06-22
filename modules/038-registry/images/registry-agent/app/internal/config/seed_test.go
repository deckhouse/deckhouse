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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSeedMirror(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bootstrap-seed.yaml")
	content := "host: 127.0.0.1:5010\nscheme: https\nca: |\n  -----BEGIN CERTIFICATE-----\n  MIIB\n  -----END CERTIFICATE-----\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSeedMirror(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatalf("got nil, want a SeedMirror")
	}
	if got.URL != "https://127.0.0.1:5010" {
		t.Fatalf("URL = %q, want https://127.0.0.1:5010", got.URL)
	}
	if got.CA == "" {
		t.Fatalf("CA empty, want PEM")
	}
}

func TestLoadSeedMirrorAbsentFileIsNil(t *testing.T) {
	got, err := LoadSeedMirror(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("absent file must not error, got %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil for absent file", got)
	}
}

func TestLoadSeedMirrorEmptyPathIsNil(t *testing.T) {
	got, err := LoadSeedMirror("")
	if err != nil || got != nil {
		t.Fatalf("empty path: got (%+v, %v), want (nil, nil)", got, err)
	}
}
