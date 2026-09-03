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

// Fuzz harnesses for the registry-proxy configuration reloader.
//
// fileContentsEqual is the only guard that decides whether the node load
// balancer configuration is revalidated and reloaded. bashible rewrites
// nginx_new.conf in place with `cp -f -p`, and the reloader reacts to inotify
// events on the directory, so it can observe a partially written file
// (registry-threat-model.md, TM-22 / AS-20).
//
// The harnesses assert:
//
//   - fileContentsEqual reports equality exactly when the bytes are equal, for
//     arbitrary content including truncated and empty files.
//   - it never reports equality when a file is missing, so a configuration is
//     never considered up to date because it could not be read.
//   - copyFile reproduces content byte for byte.
package src

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, dir, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
	return path
}

// FuzzFileContentsEqual drives the equality check with arbitrary pairs of file
// contents, including the truncated prefixes an in-place rewrite produces.
func FuzzFileContentsEqual(f *testing.F) {
	// Pairs that are equal, pairs that differ only where a size comparison
	// would miss it, and the shapes a partially written file leaves behind.
	f.Add([]byte(""), []byte(""))
	f.Add([]byte("stream {}\n"), []byte("stream {}\n"))
	f.Add([]byte("stream {}\n"), []byte("stream {}"))
	f.Add([]byte("stream {}"), []byte("stream {}\n"))
	f.Add([]byte(""), []byte("stream {}\n"))
	f.Add([]byte("stream {}\n"), []byte(""))
	f.Add([]byte("a"), []byte("b"))
	f.Add([]byte("same-length-1"), []byte("same-length-2"))
	f.Add([]byte("same-length-1"), []byte("same-length-1"))
	f.Add([]byte("\x00\x00"), []byte("\x00"))
	f.Add([]byte("\x00"), []byte("\x00"))
	f.Add([]byte("a\x00b"), []byte("a\x00c"))
	f.Add([]byte("upstream registry {\n  server 10.0.0.1:5001;\n}\n"), []byte("upstream registry {\n"))
	f.Add([]byte("server 10.0.0.1:5001;\n"), []byte("server 10.0.0.2:5001;\n"))
	f.Add([]byte("\r\n"), []byte("\n"))
	f.Add([]byte("x"), bytes.Repeat([]byte("x"), 2))
	f.Add(bytes.Repeat([]byte("x"), 1<<16), bytes.Repeat([]byte("x"), 1<<16))
	f.Add(bytes.Repeat([]byte("x"), 1<<16), bytes.Repeat([]byte("x"), (1<<16)-1))
	f.Add(bytes.Repeat([]byte("x"), 4096), bytes.Repeat([]byte("y"), 4096))
	f.Add(append(bytes.Repeat([]byte("x"), 4095), 'y'), append(bytes.Repeat([]byte("x"), 4095), 'z'))

	f.Fuzz(func(t *testing.T, current, updated []byte) {
		dir := t.TempDir()

		currentPath := writeTemp(t, dir, "nginx.conf", current)
		updatedPath := writeTemp(t, dir, "nginx_new.conf", updated)

		equal, err := fileContentsEqual(currentPath, updatedPath)
		if err != nil {
			t.Fatalf("fileContentsEqual failed on readable files: %v", err)
		}
		if want := bytes.Equal(current, updated); equal != want {
			t.Fatalf("fileContentsEqual = %v, want %v for %d and %d bytes",
				equal, want, len(current), len(updated))
		}

		// Symmetric: the caller's argument order must not matter.
		reversed, err := fileContentsEqual(updatedPath, currentPath)
		if err != nil {
			t.Fatalf("fileContentsEqual failed on readable files (reversed): %v", err)
		}
		if reversed != equal {
			t.Fatalf("fileContentsEqual is not symmetric: %v then %v", equal, reversed)
		}

		// A truncated read of the updated file must not be reported as equal to the
		// full one, which is what a partially completed `cp -f -p` looks like.
		if len(updated) > 1 {
			partialPath := writeTemp(t, dir, "nginx_new.partial.conf", updated[:len(updated)-1])
			partialEqual, err := fileContentsEqual(updatedPath, partialPath)
			if err != nil {
				t.Fatalf("fileContentsEqual failed on a truncated file: %v", err)
			}
			if partialEqual {
				t.Fatalf("fileContentsEqual reported a truncated file as equal to the full one")
			}
		}

		// A missing file must never be reported as equal: treating an unreadable
		// configuration as up to date would skip the reload silently.
		missingPath := filepath.Join(dir, "absent.conf")
		missingEqual, err := fileContentsEqual(currentPath, missingPath)
		if err == nil {
			t.Fatalf("fileContentsEqual returned no error for a missing file")
		}
		if missingEqual {
			t.Fatalf("fileContentsEqual reported a missing file as equal")
		}
	})
}

// FuzzCopyFile asserts that the copy the reloader performs before signalling
// NGINX reproduces the validated content exactly. Any divergence would mean the
// configuration NGINX loads is not the one `nginx -t` approved.
func FuzzCopyFile(f *testing.F) {
	// The file the copy has to reproduce byte for byte: the generated
	// configuration, the states an interrupted write leaves, and content that
	// is not text at all.
	f.Add([]byte("stream {\n  upstream registry {\n    server 10.0.0.1:5001;\n  }\n}\n"))
	f.Add([]byte("user deckhouse;\nevents { worker_connections 16384; }\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n"))
	f.Add([]byte(" "))
	f.Add([]byte("stream {"))
	f.Add([]byte("}"))
	f.Add([]byte("# comment only\n"))
	f.Add([]byte("\x00"))
	f.Add([]byte("\x00\xff\xfe"))
	f.Add([]byte("\xff\xfe\x00\x01\x02\x03"))
	f.Add([]byte("\r\n\r\n"))
	f.Add([]byte("a"))
	f.Add(bytes.Repeat([]byte("y"), 1<<10))
	f.Add(bytes.Repeat([]byte("y"), 1<<17))
	f.Add(bytes.Repeat([]byte("\x00"), 1<<12))
	f.Add(bytes.Repeat([]byte("server 10.0.0.1:5001;\n"), 512))
	f.Add([]byte("server ${discovered_node_ip}:5001;\n"))
	f.Add([]byte("server $(id):5001;\n"))
	f.Add(append([]byte("stream {\n"), bytes.Repeat([]byte("x"), 4096)...))

	f.Fuzz(func(t *testing.T, content []byte) {
		dir := t.TempDir()

		source := writeTemp(t, dir, "nginx_new.conf", content)
		destination := filepath.Join(dir, "nginx.conf")

		if err := copyFile(source, destination); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}

		got, err := os.ReadFile(destination)
		if err != nil {
			t.Fatalf("cannot read the copy: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("copyFile altered the content: wrote %d bytes, read %d bytes", len(content), len(got))
		}

		equal, err := fileContentsEqual(source, destination)
		if err != nil {
			t.Fatalf("fileContentsEqual failed after copyFile: %v", err)
		}
		if !equal {
			t.Fatalf("fileContentsEqual disagrees with copyFile for %d bytes", len(content))
		}

		// Overwriting an existing destination must also be exact.
		if err := copyFile(source, destination); err != nil {
			t.Fatalf("copyFile failed on overwrite: %v", err)
		}
		if got, err = os.ReadFile(destination); err != nil {
			t.Fatalf("cannot read the overwritten copy: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("copyFile altered the content on overwrite")
		}
	})
}
