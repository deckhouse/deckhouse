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

// Fuzz harness for the reload logic of registry-proxy-reloader.
//
// Threat model coverage (registry-threat-model.md), harness 9: the input file
// nginx_new.conf as watcher.go receives it -- partially written, truncated and
// syntactically invalid variants. The companion targets in fuzz_test.go cover
// file_comparison.go; this one covers the decision built on top of it, which is
// what TM-21, TM-22 and TM-23 are about.
//
// The file arrives from the bashible step that renders it from
// registry.proxyEndpoints, and it arrives through an fsnotify event, so the
// reloader can see it while it is still being written. The invariant that
// matters is the one TM-22 names: what gets applied must be what was validated.
// nginxReload checks nginx_new.conf with `nginx -t` and then copies it, reading
// the file a second time -- so a file that changes in between is validated in
// one state and applied in another.
//
// The configuration check and the reload signal are replaced here, through the
// seam in watcher.go. Not stubbing the signal would have this test walk the
// process table of the machine it runs on and SIGHUP the first process whose
// command line mentions nginx.

package src

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// confSeeds are the shapes nginx_new.conf arrives as: what the bashible step
// renders, and what a partial or interrupted write leaves behind.
var confSeeds = []string{
	"user deckhouse;\nevents { worker_connections 16384; }\nstream {\n  upstream registry {\n    least_conn;\n    server 10.0.0.1:5001;\n  }\n  server {\n    listen 127.0.0.1:5001;\n    proxy_pass registry;\n  }\n}\n",
	"",
	"user deckhouse;\n",
	"user deckhouse;\nevents { worker_connections 16384; }\nstream {\n  upstream registry {\n    least_conn;\n",
	"stream {\n",
	"}\n",
	"\n\n\n",
	"# comment only\n",
	"user deckhouse;\x00\n",
	"user deckhouse;\r\nevents {}\r\n",
	strings.Repeat("server 10.0.0.1:5001;\n", 512),
}

func FuzzNginxReload(f *testing.F) {
	for _, current := range confSeeds {
		for _, incoming := range confSeeds {
			f.Add(current, incoming, true)
			f.Add(current, incoming, false)
		}
	}

	f.Fuzz(func(t *testing.T, current, incoming string, configValid bool) {
		if len(current) > 1<<16 || len(incoming) > 1<<16 {
			return
		}

		dir := t.TempDir()

		restore := useTestPaths(t, dir)
		defer restore()

		// A configuration check whose verdict the fuzzer chooses, and which
		// records the bytes it was shown. That recording is the whole point: the
		// oracle compares what was validated with what was applied.
		var validated []byte
		var validatedRead bool

		testConfig = func(path string) ([]byte, error) {
			content, err := os.ReadFile(path)
			if err != nil {
				return []byte("cannot read"), err
			}
			validated = content
			validatedRead = true

			if !configValid {
				return []byte("test failed"), errors.New("configuration test failed")
			}
			return []byte("syntax is ok"), nil
		}

		var signalled int
		signalReload = func() error {
			signalled++
			return nil
		}

		// The current state on disk. An absent nginx.conf is a first start.
		haveCurrent := current != ""
		if haveCurrent {
			writeFile(t, nginxConf, current)
		}
		writeFile(t, nginxNewConf, incoming)

		err := nginxReload()

		applied, appliedErr := os.ReadFile(nginxConf)
		if appliedErr != nil && !os.IsNotExist(appliedErr) {
			t.Fatalf("cannot read the applied configuration: %v", appliedErr)
		}

		switch {
		case err != nil:
			// A rejected configuration must not have been signalled. Whether the
			// file was copied is a separate matter: on a first start nginxReload
			// copies before it validates, which is why the copy is not asserted
			// away here.
			if signalled != 0 {
				t.Fatalf("nginxReload() failed with %v but still sent %d reload signal(s); "+
					"a configuration nginx rejected must never be the one it is told to load",
					err, signalled)
			}

		case validatedRead:
			// The applied bytes must be the validated bytes. This is TM-22: the
			// check and the copy read the file twice, so anything that makes them
			// disagree is a configuration applied without ever being validated.
			if string(applied) != string(validated) {
				t.Fatalf("nginxReload() applied %d bytes but validated %d; the bytes that reach "+
					"nginx must be the bytes that passed `nginx -t`\n\tvalidated: %q\n\tapplied:   %q",
					len(applied), len(validated), truncate(validated), truncate(applied))
			}
			if string(applied) != incoming {
				t.Fatalf("nginxReload() applied %q, but nginx_new.conf held %q",
					truncate(applied), truncate([]byte(incoming)))
			}

		default:
			// No validation ran, so nothing may have been signalled: the only
			// path that skips the check is the one where the two files are
			// already equal.
			if signalled != 0 {
				t.Fatalf("nginxReload() sent %d reload signal(s) without running the "+
					"configuration check", signalled)
			}
			if haveCurrent && current != incoming {
				t.Fatalf("nginxReload() skipped the configuration check although nginx.conf and "+
					"nginx_new.conf differ:\n\tcurrent:  %q\n\tincoming: %q",
					truncate([]byte(current)), truncate([]byte(incoming)))
			}
		}

		// Whatever happened, at most one reload per call: each extra SIGHUP is
		// another worker cycle on a node's pull path.
		if signalled > 1 {
			t.Fatalf("nginxReload() sent %d reload signals in one call", signalled)
		}
	})
}

// useTestPaths points the reloader at a temporary directory and restores the
// package state afterwards.
func useTestPaths(t *testing.T, dir string) func() {
	t.Helper()

	confBefore, newBefore := nginxConf, nginxNewConf
	testBefore, signalBefore := testConfig, signalReload

	nginxConf = filepath.Join(dir, "nginx.conf")
	nginxNewConf = filepath.Join(dir, "nginx_new.conf")

	return func() {
		nginxConf, nginxNewConf = confBefore, newBefore
		testConfig, signalReload = testBefore, signalBefore
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
}

func truncate(content []byte) string {
	const limit = 200
	if len(content) > limit {
		return string(content[:limit]) + fmt.Sprintf("...(%d bytes)", len(content))
	}
	return string(content)
}

// TestNginxMasterSelection pins which process SIGHUP is meant for.
//
// TM-21 is about choosing the wrong one. sendReloadSignal stops at the first
// process it matches, so a match on anything other than the master means the
// master is never signalled -- and because nginxReload has already copied the
// file by then, the two files agree and every later event skips the reload too.
// The configuration is applied on disk and never loaded.
func TestNginxMasterSelection(t *testing.T) {
	cases := []struct {
		cmdline string
		master  bool
	}{
		// The master, before and after nginx rewrites its process title.
		{cmdline: "nginx: master process /opt/nginx-static/sbin/nginx -g daemon off;", master: true},
		{cmdline: "/opt/nginx-static/sbin/nginx -g daemon off;", master: true},
		{cmdline: "nginx", master: true},

		// A worker. SIGHUP here means "shut down gracefully", not "reload".
		{cmdline: "nginx: worker process"},
		{cmdline: "nginx: cache manager process"},

		// The check this reloader spawns itself, and the other one-shot forms.
		{cmdline: "nginx -t -c /etc/nginx/config/nginx_new.conf -e /dev/stderr"},
		{cmdline: "/opt/nginx-static/sbin/nginx -s reload"},
		{cmdline: "nginx -v"},

		// Processes that merely mention the word.
		{cmdline: "tail -f /var/log/nginx/error.log"},
		{cmdline: "vim /etc/nginx/config/nginx_new.conf"},
		{cmdline: "/bin/sh -c 'echo nginx'"},
		{cmdline: "grep nginx"},
		{cmdline: ""},
	}

	for _, c := range cases {
		if got := isNginxMasterCmdline(c.cmdline); got != c.master {
			t.Errorf("isNginxMasterCmdline(%q) = %v, expected %v", c.cmdline, got, c.master)
		}
	}
}
