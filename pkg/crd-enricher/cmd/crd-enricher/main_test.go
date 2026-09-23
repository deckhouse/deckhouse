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

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The library is covered by its own tests; what is covered here is the half the
// modules actually consume. Every repository using this tool invokes the binary
// through `go tool`, never the package, so the CLI's stderr and its exit code are
// the only channels they have.

// fixtureRoot is the enricher package directory, which holds the testdata the
// library tests use. The command lives one level down, so paths are relative to
// it.
const fixtureRoot = "../.."

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = saved })

	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	os.Stderr = saved
	return <-done
}

// crdDirWith copies one CRD fixture into a fresh directory, since the enricher
// rewrites files in place.
func crdDirWith(t *testing.T, fixture string) string {
	t.Helper()

	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(fixtureRoot, "testdata", "crd", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fixture), src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return dir
}

// baseArgs points the command at the library's fixture package and the given CRD
// directory.
func baseArgs(crdDir string) []string {
	return []string{
		"paths=./testdata/api/v1alpha1",
		"crds=" + crdDir,
		"dir=" + fixtureRoot,
	}
}

// TestRunPrintsWarningsAndSucceeds pins the contract of the warning channel: the
// problem reaches stderr, and the run still succeeds so adopting the tool cannot
// turn a generation step red on its own.
func TestRunPrintsWarningsAndSucceeds(t *testing.T) {
	crdDir := crdDirWith(t, "quoting.yaml")

	var err error
	stderr := captureStderr(t, func() { err = run(baseArgs(crdDir)) })

	if err != nil {
		t.Fatalf("run() = %v, want nil: a warning must not be fatal by default", err)
	}
	if !strings.Contains(stderr, "warning: ") {
		t.Fatalf("stderr carried no warning: %q", stderr)
	}
	if !strings.Contains(stderr, "parsed as a mapping") {
		t.Errorf("stderr does not name the problem: %q", stderr)
	}
	// Without the location the message cannot be acted on in a repository where
	// the same marker appears in a dozen CRDs.
	if !strings.Contains(stderr, "quoting.yaml") {
		t.Errorf("stderr does not name the manifest: %q", stderr)
	}
	if !strings.Contains(stderr, "QuotingSpec.Phase") {
		t.Errorf("stderr does not name the Go declaration: %q", stderr)
	}
}

// TestRunStrictFailsOnWarnings pins the opt-in exit code. A re-render gate over a
// committed crds/ cannot see a marker that never worked, so this is the only way
// for CI to fail on one.
func TestRunStrictFailsOnWarnings(t *testing.T) {
	crdDir := crdDirWith(t, "quoting.yaml")

	var err error
	stderr := captureStderr(t, func() { err = run(append(baseArgs(crdDir), "strict")) })

	if err == nil {
		t.Fatal("run() = nil, want an error: strict must fail when a warning was printed")
	}
	if !strings.Contains(stderr, "parsed as a mapping") {
		t.Errorf("strict must still print the warning it failed on: %q", stderr)
	}
}

// TestRunStrictSucceedsWithoutWarnings keeps strict from becoming a blanket
// failure: a clean run passes with it on, which is what makes it usable in CI.
func TestRunStrictSucceedsWithoutWarnings(t *testing.T) {
	crdDir := crdDirWith(t, "bare.yaml")

	var err error
	stderr := captureStderr(t, func() { err = run(append(baseArgs(crdDir), "strict")) })

	if err != nil {
		t.Fatalf("run() = %v, want nil: nothing warned about this fixture", err)
	}
	if strings.Contains(stderr, "warning: ") {
		t.Errorf("unexpected warning: %q", stderr)
	}
}

// TestRunArgumentParsing covers the argument table, which had no tests at all:
// every branch either sets an option or is a usage error, and a silent
// misparse would look like the tool doing nothing.
func TestRunArgumentParsing(t *testing.T) {
	crdDir := crdDirWith(t, "bare.yaml")

	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "the controller-gen output alias is accepted for crds=",
			args: []string{"paths=./testdata/api/v1alpha1", "dir=" + fixtureRoot,
				"output:crd:artifacts:config=" + crdDir},
		},
		{
			name: "quoted values are unwrapped",
			args: []string{`paths="./testdata/api/v1alpha1"`, `dir="` + fixtureRoot + `"`,
				`crds="` + crdDir + `"`},
		},
		{
			name: "comma separated paths",
			args: append(baseArgs(crdDir), "auto-examples=false", "reindent=false", "strict=false"),
		},
		{
			name:    "an unknown argument is refused rather than ignored",
			args:    append(baseArgs(crdDir), "nonsense=1"),
			wantErr: `unknown argument "nonsense=1"`,
		},
		{
			name:    "a malformed boolean is refused",
			args:    append(baseArgs(crdDir), "reindent=perhaps"),
			wantErr: `invalid boolean value "perhaps"`,
		},
		{
			name:    "paths is required",
			args:    []string{"crds=" + crdDir},
			wantErr: "no package paths provided",
		},
		{
			name:    "crds is required",
			args:    []string{"paths=./testdata/api/v1alpha1", "dir=" + fixtureRoot},
			wantErr: "no CRD directory provided",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			captureStderr(t, func() { err = run(tc.args) })

			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("run() = %v, want nil", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("run() = nil, want an error containing %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("run() = %v, want an error containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestRunHelp keeps the help path from reaching the enricher: it takes no
// arguments and must not need them.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			if err := run([]string{arg}); err != nil {
				t.Errorf("run(%q) = %v, want nil", arg, err)
			}
		})
	}
}
