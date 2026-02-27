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

package oss_yaml_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	cur := wd
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			t.Fatalf("repo root not found from %s", wd)
		}
		cur = next
	}
}

func ensureWerfBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	werf := filepath.Join(repoRoot, "bin", "werf")
	if runtime.GOOS == "windows" {
		werf += ".exe"
	}
	if _, err := os.Stat(werf); err == nil {
		return werf
	}

	cmd := exec.Command("make", "bin/werf")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("werf binary not found and failed to build via `make bin/werf`: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(werf); err != nil {
		t.Skipf("werf binary still not found after build at %s", werf)
	}
	return werf
}

func syncHelperIntoCaseDir(t *testing.T, repoRoot, caseDir string) {
	t.Helper()

	src := filepath.Join(repoRoot, ".werf", "defines", "oss-yaml.tmpl")
	dstDir := filepath.Join(caseDir, ".werf", "defines")
	dst := filepath.Join(dstDir, "oss-yaml.tmpl")

	// Ensure clean state (remove possible symlink leftover).
	_ = os.RemoveAll(dstDir)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dstDir, err)
	}

	// Prefer symlink to always test the current helper implementation.
	// On Windows symlinks might require special privileges, so we fallback to copying.
	if runtime.GOOS != "windows" {
		rel, err := filepath.Rel(dstDir, src)
		if err != nil {
			t.Fatalf("rel symlink path from %s to %s: %v", dstDir, src, err)
		}
		if err := os.Symlink(rel, dst); err != nil {
			t.Fatalf("symlink %s -> %s: %v", dst, rel, err)
		}
		return
	}

	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read helper template %s: %v", src, err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatalf("write helper template %s: %v", dst, err)
	}
}

func runWerfRender(t *testing.T, repoRoot, werf, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command(werf, "config", "render", "--dev")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"WERF_LOG_COLOR_MODE=off",
		"WERF_LOG_VERBOSE=false",
		"WERF_GITERMINISM_CONFIG="+filepath.Join(repoRoot, "werf-giterminism.yaml"),
	)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func listCaseDirs(t *testing.T, base string) []string {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("readdir %s: %v", base, err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirs = append(dirs, filepath.Join(base, e.Name()))
	}
	if len(dirs) == 0 {
		t.Fatalf("no testcases found in %s", base)
	}
	return dirs
}

func TestWerfDefinesOssYamlHelpers_RenderOK(t *testing.T) {
	repoRoot := findRepoRoot(t)
	werf := ensureWerfBinary(t, repoRoot)
	base := filepath.Join(repoRoot, "testing", "werf_defines", "oss-yaml", "render-ok")

	for _, d := range listCaseDirs(t, base) {
		name := filepath.Base(d)
		t.Run(name, func(t *testing.T) {
			syncHelperIntoCaseDir(t, repoRoot, d)
			out, err := runWerfRender(t, repoRoot, werf, d)
			if err != nil {
				t.Fatalf("expected render OK, got error: %v\n--- output ---\n%s", err, out)
			}
		})
	}
}

func TestWerfDefinesOssYamlHelpers_RenderFail(t *testing.T) {
	repoRoot := findRepoRoot(t)
	werf := ensureWerfBinary(t, repoRoot)
	base := filepath.Join(repoRoot, "testing", "werf_defines", "oss-yaml", "render-fail")

	for _, d := range listCaseDirs(t, base) {
		name := filepath.Base(d)
		t.Run(name, func(t *testing.T) {
			syncHelperIntoCaseDir(t, repoRoot, d)
			out, err := runWerfRender(t, repoRoot, werf, d)
			if err == nil {
				t.Fatalf("expected render to fail, but it succeeded\n--- output ---\n%s", out)
			}
		})
	}
}
