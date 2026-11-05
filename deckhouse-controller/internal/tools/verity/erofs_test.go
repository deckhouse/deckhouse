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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
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

// TestCreateImageByTarFailedStream guards against waiting for mkfs.erofs forever
// after the tar stream breaks: the read error must kill mkfs.erofs and come back
// as the cause, whether mkfs.erofs keeps running on the truncated tar or fails on it.
func TestCreateImageByTarFailedStream(t *testing.T) {
	errStream := errors.New("stream broken")

	for _, tc := range []struct {
		name     string
		afterEOF string
		err      error
	}{
		{name: "mkfs.erofs keeps running", afterEOF: "exec sleep 60", err: errStream},
		{name: "mkfs.erofs fails", afterEOF: "exit 1", err: errStream},
		// net/http reports a connection closed by the registry as a wrapped io.EOF
		{name: "wrapped io.EOF", afterEOF: "exec sleep 60", err: fmt.Errorf("read layer: %w", io.EOF)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeMkfs(t, tc.afterEOF)

			// kills mkfs.erofs if CreateImageByTar does not
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			imagePath := filepath.Join(t.TempDir(), "main.erofs")
			stream := io.NopCloser(io.MultiReader(strings.NewReader("partial tar"), iotest.ErrReader(tc.err)))

			done := make(chan error, 1)
			go func() {
				done <- CreateImageByTar(ctx, stream, imagePath)
			}()

			select {
			case err := <-done:
				if !errors.Is(err, ErrImageStream) || !errors.Is(err, tc.err) {
					t.Fatalf("CreateImageByTar() error = %v, want the stream error", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("CreateImageByTar() still waits for mkfs.erofs after the stream failed")
			}
		})
	}
}

// TestCreateImageByTarCompleteStream checks that a clean end of the stream lets
// mkfs.erofs finish with the whole tar.
func TestCreateImageByTarCompleteStream(t *testing.T) {
	fakeMkfs(t, "exit 0")

	imagePath := filepath.Join(t.TempDir(), "main.erofs")
	if err := CreateImageByTar(context.Background(), io.NopCloser(strings.NewReader("whole tar")), imagePath); err != nil {
		t.Fatalf("CreateImageByTar() error = %v", err)
	}

	got, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	if string(got) != "whole tar" {
		t.Fatalf("mkfs.erofs got %q, want the whole tar", got)
	}
}

// fakeMkfs puts a mkfs.erofs on PATH that copies stdin to the image path,
// the last argument, and then runs afterEOF.
func fakeMkfs(t *testing.T, afterEOF string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\nfor image; do :; done\ncat > \"$image\"\n" + afterEOF + "\n"
	if err := os.WriteFile(filepath.Join(dir, mkfsCommand), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", mkfsCommand, err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
