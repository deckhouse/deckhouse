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

package crdenricher

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden is bound to the shared -golden test flag, matching the deckhouse
// convention: `go test ./... -golden` regenerates the golden files instead of
// comparing against them.
var updateGolden = flag.Bool("golden", false, "regenerate golden files instead of comparing against them")

// assertGolden compares got against testdata/golden/<name>, or rewrites that
// file when -golden is set. The comparison is byte-exact so the key ordering the
// enricher deliberately controls (examples vs sorted schema) is part of the
// assertion — a semantic YAML comparison would miss it.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run `go test -golden` to generate it): %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("result does not match golden %s (run `go test -golden` to update)\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got)
	}
}
