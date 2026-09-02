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

//go:build unix

package bootstrap

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

// TestClaimLegacyCacheDropsAMarkerItCouldNotWrite pins the recovery from a half-made claim. The
// marker is created with O_EXCL, so a file left holding less than the whole identity is one that
// matches no cluster and that no later run may rewrite: the upgrade-era cache behind it would be
// unadoptable by anyone, forever, with nothing in the output to say why.
//
// The write is failed with RLIMIT_FSIZE, the same class of failure as the ENOSPC and the rw->ro
// remount this is about: the create succeeds, the write does not.
func TestClaimLegacyCacheDropsAMarkerItCouldNotWrite(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	legacy := (&config.MetaConfig{ClusterType: config.StaticClusterType}).CachePath()
	legacyDir := filepath.Join(dir, stringsutil.Sha256Encode(legacy))
	marker := legacyDir + ".owner"

	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "uuid"), []byte("u"), 0o600))

	withZeroFileSizeLimit(t, func() {
		require.False(t, claimLegacyCache(ctx, dir, legacy, "ssh-first"),
			"a claim that could not be recorded is not a claim",
		)
	})

	_, err := os.Stat(marker)
	require.True(t, os.IsNotExist(err), "the marker the write failed on has to go with it")

	require.True(t, claimLegacyCache(ctx, dir, legacy, "ssh-second"),
		"the directory has to stay adoptable once the disk lets go",
	)

	owner, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "ssh-second", string(owner))
}

// withZeroFileSizeLimit runs fn with a file size limit of zero, under which a write to a regular
// file fails with EFBIG. The limit is process-wide, so the window holds nothing but fn: Go leaves
// the SIGXFSZ that comes with the error unhandled, and test output goes to a pipe, which the
// limit does not cover.
func withZeroFileSizeLimit(t *testing.T, fn func()) {
	t.Helper()

	var lim syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &lim))

	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Max: lim.Max}))
	defer func() { require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim)) }()

	fn()
}
