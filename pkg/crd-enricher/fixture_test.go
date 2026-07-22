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

// This file holds the shared fixture driver for the end-to-end enricher tests.
// Every feature has its own <name>.go API struct, <name>.yaml CRD, <name>_test.go
// test and testdata/golden/<name>.yaml golden snapshot; runFixture drives a
// single fixture through Run and returns the enriched bytes for assertGolden.

import (
	"os"
	"path/filepath"
	"testing"
)

const testFixturePaths = "./testdata/api/v1alpha1"

// runFixture copies a single CRD fixture into a fresh temp directory (Run
// rewrites files in place) and runs the enricher over it, returning the
// enriched file bytes. Each fixture backs exactly one root type, so a test runs
// the enricher against exactly one feature.
func runFixture(t *testing.T, crdFile string, generateExamples bool) []byte {
	t.Helper()

	crdDir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "crd", crdFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(crdDir, crdFile)
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}

	if _, err := Run(Options{
		Paths:            []string{testFixturePaths},
		CRDDir:           crdDir,
		GenerateExamples: generateExamples,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read enriched fixture: %v", err)
	}
	return out
}
