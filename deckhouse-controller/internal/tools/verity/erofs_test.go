/*
Copyright 2025 Flant JSC

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

package verity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSelfTestDMVerityDoesNotHang guards against the self-test regressing
// into an unbounded wait: on a host without dm-verity support (or without
// the veritysetup/mkfs.erofs binaries at all, as in this CI/dev environment)
// it must return false quickly, not hang for the full previous 15-30 minute
// install timeout the original bug caused downstream.
func TestSelfTestDMVerityDoesNotHang(t *testing.T) {
	start := time.Now()
	got := selfTestDMVerity()
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Fatalf("selfTestDMVerity took %s, want it bounded well under the 5s self-test timeout", elapsed)
	}

	t.Logf("selfTestDMVerity() = %v, took %s", got, elapsed)
}

// TestSelfTestDMVerityCleansUpTempFiles ensures the self-test never leaks its
// scratch directory, whether it succeeds or fails.
func TestSelfTestDMVerityCleansUpTempFiles(t *testing.T) {
	before := countSelftestTempDirs(t)

	selfTestDMVerity()

	after := countSelftestTempDirs(t)
	if after != before {
		t.Fatalf("selfTestDMVerity leaked a temp dir: before=%d after=%d", before, after)
	}
}

// TestIsSupportedIsMemoized calls IsSupported twice and expects the second
// call to be effectively free (sync.OnceValue), and both calls to agree.
func TestIsSupportedIsMemoized(t *testing.T) {
	first := IsSupported()

	start := time.Now()
	second := IsSupported()
	elapsed := time.Since(start)

	if first != second {
		t.Fatalf("IsSupported() is not stable across calls: first=%v second=%v", first, second)
	}

	if elapsed > 100*time.Millisecond {
		t.Fatalf("second IsSupported() call took %s, want it memoized (near-instant)", elapsed)
	}
}

func countSelftestTempDirs(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(filepath.Base(e.Name()), "verity-selftest-") {
			count++
		}
	}

	return count
}
