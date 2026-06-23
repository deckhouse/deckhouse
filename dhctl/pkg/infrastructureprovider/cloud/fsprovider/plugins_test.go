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

package fsprovider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyTFVersionFileCopiesVersionsAndPlanRules(t *testing.T) {
	root := t.TempDir()
	tfManagerDir := filepath.Join(root, "terraform-manager")
	require.NoError(t, os.MkdirAll(tfManagerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tfManagerDir, versionFile), []byte("terraform: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tfManagerDir, planRulesFilename), []byte("vmResource:\n  type: kubernetes_manifest\n"), 0o644))

	require.NoError(t, copyTFVersionFile(root, tfManagerDir))

	candi := filepath.Join(root, "deckhouse", "candi")
	// plan_rules must land next to terraform_versions so loadPlanRules finds it.
	require.FileExists(t, filepath.Join(candi, versionFile))
	require.FileExists(t, filepath.Join(candi, planRulesFilename))
}

func TestCopyTFVersionFilePlanRulesOptional(t *testing.T) {
	root := t.TempDir()
	tfManagerDir := filepath.Join(root, "terraform-manager")
	require.NoError(t, os.MkdirAll(tfManagerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tfManagerDir, versionFile), []byte("terraform: 1\n"), 0o644))

	// A provider bundle without plan_rules.yml must not fail the copy.
	require.NoError(t, copyTFVersionFile(root, tfManagerDir))

	candi := filepath.Join(root, "deckhouse", "candi")
	require.FileExists(t, filepath.Join(candi, versionFile))
	require.NoFileExists(t, filepath.Join(candi, planRulesFilename))
}

func TestCopyTFVersionFileMissingVersionsFails(t *testing.T) {
	root := t.TempDir()
	tfManagerDir := filepath.Join(root, "terraform-manager")
	require.NoError(t, os.MkdirAll(tfManagerDir, 0o755))

	require.Error(t, copyTFVersionFile(root, tfManagerDir))
}
