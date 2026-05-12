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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	testRootToken      = "$TEST_ROOT"
	testRootRelFromGit = "modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases"
)

func normalizeRefForSuite(ref, outDir string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", fmt.Errorf("empty ref")
	}
	abs, ok, err := resolveTokenToAbsPath(trimmed, outDir, outDir)
	if err != nil {
		return "", err
	}
	if !ok {
		return filepath.ToSlash(trimmed), nil
	}
	rel, err := filepath.Rel(outDir, abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s relative path: %w", testRootToken, err)
	}
	return filepath.ToSlash(rel), nil
}

func resolveTokenToAbsPath(relPath, sourceDir, renderedDir string) (string, bool, error) {
	clean := strings.TrimSpace(relPath)
	if !strings.HasPrefix(clean, testRootToken) {
		return "", false, nil
	}
	suffix := strings.TrimPrefix(clean, testRootToken)
	suffix = strings.TrimPrefix(suffix, "/")
	suffix = strings.TrimPrefix(suffix, "\\")

	testRootAbs, err := resolveTokenTestRoot(sourceDir, renderedDir)
	if err != nil {
		return "", true, err
	}
	target := filepath.Clean(filepath.Join(testRootAbs, filepath.FromSlash(suffix)))
	if target == testRootAbs || strings.HasPrefix(target, testRootAbs+string(os.PathSeparator)) {
		return target, true, nil
	}
	return "", true, fmt.Errorf("%s path escapes test root: %q", testRootToken, relPath)
}

func resolveTokenTestRoot(hints ...string) (string, error) {
	if envRoot := strings.TrimSpace(os.Getenv("TEST_ROOT")); envRoot != "" {
		if !filepath.IsAbs(envRoot) {
			return "", fmt.Errorf("TEST_ROOT must be absolute path, got %q", envRoot)
		}
		return filepath.Clean(envRoot), nil
	}
	gitRoot, err := resolveGitRoot(hints...)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(gitRoot, filepath.FromSlash(testRootRelFromGit))), nil
}

func resolveGitRoot(hints ...string) (string, error) {
	checked := map[string]struct{}{}
	for _, h := range hints {
		if strings.TrimSpace(h) == "" {
			continue
		}
		start := h
		if st, err := os.Stat(start); err == nil && !st.IsDir() {
			start = filepath.Dir(start)
		}
		if root, ok := findGitRootFrom(start); ok {
			return root, nil
		}
		checked[start] = struct{}{}
	}
	if cwd, err := os.Getwd(); err == nil {
		if _, seen := checked[cwd]; !seen {
			if root, ok := findGitRootFrom(cwd); ok {
				return root, nil
			}
		}
	}
	return "", fmt.Errorf("cannot resolve git root for %s token", testRootToken)
}

func findGitRootFrom(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		gitMarker := filepath.Join(dir, ".git")
		if st, err := os.Stat(gitMarker); err == nil {
			if st.IsDir() || !st.IsDir() {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
