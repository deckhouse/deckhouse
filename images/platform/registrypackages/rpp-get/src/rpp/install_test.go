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

package rpp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type packageArchiveFile struct {
	body string
	mode int64
}

func TestInstallPackageOnceHappyPath(t *testing.T) {
	client := newTestClient(t, nil)
	ref := mustPackageRef(t, client, "alpha:sha256:abc123")
	markerPath := filepath.Join(t.TempDir(), "installed-marker")

	writeArchive(t, ref.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript(
				"echo ok > \"" + markerPath + "\"",
			),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	if err := client.installPackageOnce(context.Background(), ref); err != nil {
		t.Fatalf("installPackageOnce() error = %v", err)
	}

	assertFileContent(t, filepath.Join(ref.installedDir, "digest"), ref.digest+"\n")
	assertPathExists(t, filepath.Join(ref.installedDir, "install"))
	assertPathExists(t, filepath.Join(ref.installedDir, "uninstall"))
	assertFileContent(t, markerPath, "ok\n")
	assertPathNotExists(t, ref.archivePath)
	assertPathNotExists(t, filepath.Dir(ref.archivePath))
}

func TestInstallPackageOnceSkipsAlreadyInstalled(t *testing.T) {
	client := newTestClient(t, nil)
	ref := mustPackageRef(t, client, "beta:sha256:def456")

	if err := os.MkdirAll(ref.installedDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(installedDir) error = %v", err)
	}
	assertNoError(t, os.WriteFile(filepath.Join(ref.installedDir, "digest"), []byte(ref.digest+"\n"), 0o644))

	if err := os.MkdirAll(filepath.Dir(ref.archivePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(archiveDir) error = %v", err)
	}
	assertNoError(t, os.WriteFile(ref.archivePath, []byte("leave-me"), 0o644))

	if err := client.installPackageOnce(context.Background(), ref); err != nil {
		t.Fatalf("installPackageOnce() error = %v", err)
	}

	assertFileContent(t, filepath.Join(ref.installedDir, "digest"), ref.digest+"\n")
	assertFileContent(t, ref.archivePath, "leave-me")
}

func TestInstallPackageOnceFailsOnBrokenArchive(t *testing.T) {
	client := newTestClient(t, nil)
	ref := mustPackageRef(t, client, "gamma:sha256:broken001")

	if err := os.MkdirAll(filepath.Dir(ref.archivePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(archiveDir) error = %v", err)
	}
	assertNoError(t, os.WriteFile(ref.archivePath, []byte("not-a-tar.gz"), 0o644))

	err := client.installPackageOnce(context.Background(), ref)
	if err == nil {
		t.Fatal("installPackageOnce() error = nil, want extract error")
	}
	if !strings.Contains(err.Error(), "extract") {
		t.Fatalf("installPackageOnce() error = %v, want extract error", err)
	}

	assertPathNotExists(t, ref.installedDir)
	assertPathExists(t, ref.archivePath)
}

func TestInstallPackageOnceFailsOnInstallScript(t *testing.T) {
	client := newTestClient(t, nil)
	ref := mustPackageRef(t, client, "delta:sha256:failscript")

	writeArchive(t, ref.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 12"),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	err := client.installPackageOnce(context.Background(), ref)
	if err == nil {
		t.Fatal("installPackageOnce() error = nil, want install script error")
	}
	if !strings.Contains(err.Error(), "run install script") {
		t.Fatalf("installPackageOnce() error = %v, want install script error", err)
	}

	assertPathNotExists(t, ref.installedDir)
	assertPathExists(t, ref.archivePath)
}

func TestInstallPackageOnceFailsWhenUninstallMissing(t *testing.T) {
	client := newTestClient(t, nil)
	ref := mustPackageRef(t, client, "epsilon:sha256:missinguninstall")

	writeArchive(t, ref.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	err := client.installPackageOnce(context.Background(), ref)
	if err == nil {
		t.Fatal("installPackageOnce() error = nil, want metadata copy error")
	}
	if !strings.Contains(err.Error(), "copy uninstall") {
		t.Fatalf("installPackageOnce() error = %v, want uninstall copy error", err)
	}

	assertPathExists(t, filepath.Join(ref.installedDir, "digest"))
	assertPathExists(t, filepath.Join(ref.installedDir, "install"))
	assertPathExists(t, ref.archivePath)
}

func TestInstallPackageRetriesAndSucceeds(t *testing.T) {
	token := "test-token"
	stateDir := t.TempDir()
	firstFailMarker := filepath.Join(stateDir, "first-fail")
	successMarker := filepath.Join(stateDir, "success")
	archive := buildArchive(t, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript(
				"if [ ! -f \""+firstFailMarker+"\" ]; then",
				"  : > \""+firstFailMarker+"\"",
				"  exit 1",
				"fi",
				"echo ok > \""+successMarker+"\"",
			),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	server, count := newArchiveServer(t, token, map[string][]byte{
		"sha256:retrysuccess": archive,
	})
	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Token = token
	client.cfg.RetryDelay = 0
	client.cfg.Retries = 1
	client.httpClient = newHTTPClient(client.cfg)
	ref := mustPackageRef(t, client, "zeta:sha256:retrysuccess")

	if err := client.installPackage(context.Background(), ref); err != nil {
		t.Fatalf("installPackage() error = %v", err)
	}

	if got := count.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}

	assertFileContent(t, filepath.Join(ref.installedDir, "digest"), ref.digest+"\n")
	assertFileContent(t, successMarker, "ok\n")
	assertPathNotExists(t, filepath.Dir(ref.archivePath))
}

func TestInstallPackageFailsAfterRetriesAndCleansUp(t *testing.T) {
	token := "test-token"
	archive := buildArchive(t, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 1"),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	server, count := newArchiveServer(t, token, map[string][]byte{
		"sha256:retryfail": archive,
	})
	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Token = token
	client.cfg.RetryDelay = 0
	client.cfg.Retries = 1
	client.httpClient = newHTTPClient(client.cfg)
	ref := mustPackageRef(t, client, "eta:sha256:retryfail")

	err := client.installPackage(context.Background(), ref)
	if err == nil {
		t.Fatal("installPackage() error = nil, want retry failure")
	}
	if !strings.Contains(err.Error(), "run install script") {
		t.Fatalf("installPackage() error = %v, want install script error", err)
	}

	if got := count.Load(); got != int32(packageInstallAttempts) {
		t.Fatalf("request count = %d, want %d", got, packageInstallAttempts)
	}

	assertPathNotExists(t, ref.installedDir)
	assertPathNotExists(t, filepath.Dir(ref.archivePath))
}

func TestInstallAllHappyPath(t *testing.T) {
	client := newTestClient(t, nil)
	markerOne := filepath.Join(t.TempDir(), "one")
	markerTwo := filepath.Join(t.TempDir(), "two")

	refOne := mustPackageRef(t, client, "theta:sha256:allgood1")
	refTwo := mustPackageRef(t, client, "iota:sha256:allgood2")

	writeArchive(t, refOne.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("echo one > \"" + markerOne + "\""),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})
	writeArchive(t, refTwo.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("echo two > \"" + markerTwo + "\""),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	if err := client.InstallAll(context.Background(), []string{refOne.raw, refTwo.raw}); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}

	assertFileContent(t, filepath.Join(refOne.installedDir, "digest"), refOne.digest+"\n")
	assertFileContent(t, filepath.Join(refTwo.installedDir, "digest"), refTwo.digest+"\n")
	assertFileContent(t, markerOne, "one\n")
	assertFileContent(t, markerTwo, "two\n")
}

func TestInstallAllReturnsErrorWhenOnePackageFails(t *testing.T) {
	token := "test-token"
	goodArchive := buildArchive(t, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})
	badArchive := buildArchive(t, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 1"),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	server, _ := newArchiveServer(t, token, map[string][]byte{
		"sha256:allbad1":  badArchive,
		"sha256:allgood3": goodArchive,
	})
	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Token = token
	client.cfg.RetryDelay = 0
	client.cfg.Retries = 1
	client.httpClient = newHTTPClient(client.cfg)

	goodRef := mustPackageRef(t, client, "kappa:sha256:allgood3")
	badRef := mustPackageRef(t, client, "lambda:sha256:allbad1")

	err := client.InstallAll(context.Background(), []string{goodRef.raw, badRef.raw})
	if err == nil {
		t.Fatal("InstallAll() error = nil, want aggregated failure")
	}

	assertFileContent(t, filepath.Join(goodRef.installedDir, "digest"), goodRef.digest+"\n")
	assertPathNotExists(t, badRef.installedDir)
}

func TestInstallAllWritesResultFile(t *testing.T) {
	client := newTestClient(t, nil)
	resultPath := filepath.Join(t.TempDir(), "result.log")
	recorder, err := NewResultRecorder(resultPath)
	if err != nil {
		t.Fatalf("NewResultRecorder() error = %v", err)
	}
	client.resultRecorder = recorder

	installedRef := mustPackageRef(t, client, "mu:sha256:result001")
	skippedRef := mustPackageRef(t, client, "nu:sha256:result002")

	writeArchive(t, installedRef.archivePath, map[string]packageArchiveFile{
		"install": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
		"uninstall": {
			mode: 0o755,
			body: shellScript("exit 0"),
		},
	})

	assertNoError(t, os.MkdirAll(skippedRef.installedDir, 0o755))
	assertNoError(t, os.WriteFile(filepath.Join(skippedRef.installedDir, "digest"), []byte(skippedRef.digest+"\n"), 0o644))

	if err := client.InstallAll(context.Background(), []string{installedRef.raw, skippedRef.raw}); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}

	assertResultLines(t, resultPath, []string{
		resultInstalled + " " + installedRef.name,
		resultSkipped + " " + skippedRef.name,
	})
}

func TestUninstallAllWritesResultFile(t *testing.T) {
	client := newTestClient(t, nil)
	resultPath := filepath.Join(t.TempDir(), "result.log")
	recorder, err := NewResultRecorder(resultPath)
	if err != nil {
		t.Fatalf("NewResultRecorder() error = %v", err)
	}
	client.resultRecorder = recorder

	removedRef := packageRef{
		raw:          "xi",
		name:         "xi",
		installedDir: filepath.Join(client.cfg.InstalledStore, "xi"),
	}

	assertNoError(t, os.MkdirAll(removedRef.installedDir, 0o755))
	assertNoError(t, os.WriteFile(filepath.Join(removedRef.installedDir, "uninstall"), []byte(shellScript("exit 0")), 0o755))

	if err := client.UninstallAll(context.Background(), []string{removedRef.name, "omicron"}); err != nil {
		t.Fatalf("UninstallAll() error = %v", err)
	}

	assertResultLines(t, resultPath, []string{
		resultRemoved + " " + removedRef.name,
		resultSkipped + " omicron",
	})
}

func newTestClient(t *testing.T, endpoints []string) *Client {
	t.Helper()

	root := t.TempDir()
	cfg := Config{
		TempDir:        root,
		InstalledStore: filepath.Join(root, "installed"),
		Retries:        1,
		RetryDelay:     0,
		Endpoints:      endpoints,
		Token:          "test-token",
	}

	for _, path := range []string{root, cfg.InstalledStore, defaultFetchedStore(root)} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%s) error = %v", path, err)
		}
	}

	recorder, err := NewResultRecorder("")
	if err != nil {
		t.Fatalf("NewResultRecorder() error = %v", err)
	}

	return NewClient(cfg, log.New(io.Discard, "", 0), recorder)
}

func mustPackageRef(t *testing.T, client *Client, raw string) packageRef {
	t.Helper()

	ref, err := client.newPackageRef(raw)
	if err != nil {
		t.Fatalf("newPackageRef(%q) error = %v", raw, err)
	}

	return ref
}

func writeArchive(t *testing.T, archivePath string, files map[string]packageArchiveFile) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(archivePath), err)
	}

	if err := os.WriteFile(archivePath, buildArchive(t, files), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", archivePath, err)
	}
}

func buildArchive(t *testing.T, files map[string]packageArchiveFile) []byte {
	t.Helper()

	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, name := range names {
		file := files[name]
		header := &tar.Header{
			Name: name,
			Mode: file.mode,
			Size: int64(len(file.body)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%s) error = %v", name, err)
		}

		if _, err := tarWriter.Write([]byte(file.body)); err != nil {
			t.Fatalf("Write(%s) error = %v", name, err)
		}
	}

	assertNoError(t, tarWriter.Close())
	assertNoError(t, gzipWriter.Close())
	return buffer.Bytes()
}

func newArchiveServer(t *testing.T, token string, archives map[string][]byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var count atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)

		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		archive, ok := archives[r.URL.Query().Get("digest")]
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/x-gzip")
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)

	return server, &count
}

func serverHost(t *testing.T, server *httptest.Server) string {
	t.Helper()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", server.URL, err)
	}

	return parsed.Host
}

func shellScript(lines ...string) string {
	return "#!/bin/sh\nset -eu\n" + strings.Join(lines, "\n") + "\n"
}

func assertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%s) error = %v, want existing path", path, err)
	}
}

func assertPathNotExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("os.Stat(%s) error = nil, want path to be absent", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(%s) error = %v, want not-exist", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", path, err)
	}

	if got := string(data); got != want {
		t.Fatalf("file %s content = %q, want %q", path, got, want)
	}
}

func assertResultLines(t *testing.T, path string, want []string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", path, err)
	}

	got := strings.FieldsFunc(strings.TrimSpace(string(data)), func(r rune) bool {
		return r == '\n'
	})
	sort.Strings(got)
	sort.Strings(want)

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("result lines = %q, want %q", got, want)
	}
}

// Ensure time.Duration zero value works for RetryDelay.
var _ = time.Duration(0)
