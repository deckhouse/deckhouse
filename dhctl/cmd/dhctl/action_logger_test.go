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
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// TestRootWritesFileAndTTY pins that the debug-log pointer reaches the terminal as well as the
// file. It is tagged for the compact view the same way initLogger tags it: untagged Info is
// ordinary detail and a terminal shows the curated view, so without the tag the one message a user
// needs when something goes wrong would be the one they never see.
func TestRootWritesFileAndTTY(t *testing.T) {
	var file, tty bytes.Buffer
	root := logger.NewRoot(logger.Options{FileWriter: &file, TTYWriter: &tty, IsTTY: true})
	root.LogAttrs(context.Background(), slog.LevelInfo, "Debug log file: /tmp/x.log", logger.ShowInCompacted())

	if !strings.Contains(file.String(), "Debug log file") {
		t.Fatalf("file missing notice: %q", file.String())
	}
	if !strings.Contains(tty.String(), "Debug log file") {
		t.Fatalf("tty missing notice: %q", tty.String())
	}
}

// Environment errors arrive before the real logger: they must read as text on stderr, not as
// debug-file JSON.
func TestFallbackLoggerWritesText(t *testing.T) {
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)

	orig := os.Stderr
	os.Stderr = stderr

	t.Cleanup(func() { os.Stderr = orig })

	l := newActionIniter(options.New()).Logger()
	l.Debug("bootstrap")
	l.Error(`--resources-timeout: time: invalid duration "x"`)

	out, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	require.Contains(t, string(out), `--resources-timeout: time: invalid duration "x"`)
	require.NotContains(t, string(out), `"level"`)
	require.NotContains(t, string(out), "bootstrap")
}
