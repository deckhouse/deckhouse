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

package docs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// BenchmarkRenderVersions renders, one by one, the module versions whose
// documentation archives (<module>@<version>.tar, as ondemand.DocsArchive
// writes them) are in $DOCS_BUILDER_BENCH_ARCHIVES.
func BenchmarkRenderVersions(b *testing.B) {
	files, _ := filepath.Glob(filepath.Join(os.Getenv("DOCS_BUILDER_BENCH_ARCHIVES"), "*.tar"))
	if len(files) == 0 {
		b.Skip("no archives in $DOCS_BUILDER_BENCH_ARCHIVES")
	}
	archives := make(map[string][]byte, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		archives[strings.TrimSuffix(filepath.Base(f), ".tar")] = raw
	}

	for b.Loop() {
		for name, raw := range archives {
			module, version, _ := strings.Cut(name, "@")
			dir := b.TempDir()
			if err := RenderVersion(context.Background(), "../../../docs-builder-template", filepath.Join(dir, "site"), filepath.Join(dir, "out"), module, version, bytes.NewReader(raw), false, log.NewNop()); err != nil {
				b.Fatal(name, err)
			}
		}
	}
}
