// Copyright 2025 Flant JSC
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
)

// resolveTestsRoot returns a base root that contains constraint test sets.
// Supports both legacy layout with profiles/ and per-constraint layout with local test_profile.yaml.
func resolveTestsRoot(testsRoot string) (baseRoot, profilesDir string, err error) {
	profilesDir = filepath.Join(testsRoot, "profiles")
	if st, e := os.Stat(profilesDir); e == nil && st.IsDir() {
		return testsRoot, profilesDir, nil
	}

	candidate := filepath.Join(testsRoot, "constraints")
	profilesDir = filepath.Join(candidate, "profiles")
	if st, e := os.Stat(profilesDir); e == nil && st.IsDir() {
		return candidate, profilesDir, nil
	}

	parent := filepath.Dir(testsRoot)
	profilesDir = filepath.Join(parent, "profiles")
	if st, e := os.Stat(profilesDir); e == nil && st.IsDir() {
		return parent, profilesDir, nil
	}

	if hasAnyPerConstraintProfile(testsRoot) {
		return testsRoot, "", nil
	}
	if hasAnyPerConstraintProfile(candidate) {
		return candidate, "", nil
	}
	if hasAnyPerConstraintProfile(parent) {
		return parent, "", nil
	}

	return "", "", fmt.Errorf("test profiles not found under %s", testsRoot)
}

func hasAnyPerConstraintProfile(root string) bool {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return false
	}
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "rendered" || d.Name() == "constraints" || d.Name() == "test_samples" {
				return nil
			}
			return nil
		}
		if filepath.Base(path) == "test_profile.yaml" {
			found = true
			return filepath.SkipDir
		}
		return nil
	})
	return found
}
