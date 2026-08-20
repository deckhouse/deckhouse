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

package bashible

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeLocalStep(t *testing.T, bundleDir, name, content string) string {
	t.Helper()

	path := filepath.Join(bundleDir, localBundleStepsDir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return checksumOf(content)
}

func checksumOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestVerifyStepsStatus_NoSavedEntries(t *testing.T) {
	bundleDir := t.TempDir()
	require.NoError(t, VerifyStepsStatus(bundleDir, nil))
}

func TestVerifyStepsStatus_UnchangedStepsMatch(t *testing.T) {
	bundleDir := t.TempDir()
	checksum := writeLocalStep(t, bundleDir, "000_step_one", "echo hello\n")

	err := VerifyStepsStatus(bundleDir, map[string]string{"000_step_one": checksum})
	require.NoError(t, err)
}

func TestVerifyStepsStatus_ChangedContentIsRejected(t *testing.T) {
	bundleDir := t.TempDir()
	writeLocalStep(t, bundleDir, "000_step_one", "echo hello\n")

	err := VerifyStepsStatus(bundleDir, map[string]string{"000_step_one": checksumOf("echo something-else\n")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "000_step_one")
	require.Contains(t, err.Error(), "bootstrap-phase abort")
}

func TestVerifyStepsStatus_MissingStepIsRejected(t *testing.T) {
	bundleDir := t.TempDir()

	err := VerifyStepsStatus(bundleDir, map[string]string{"000_step_one": checksumOf("echo hello\n")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "000_step_one")
}
