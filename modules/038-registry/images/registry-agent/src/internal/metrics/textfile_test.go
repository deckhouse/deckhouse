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

package metrics

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func recorded(t *testing.T) *prometheus.Registry {
	t.Helper()

	registry := prometheus.NewRegistry()
	m := New(registry)
	m.Requests.WithLabelValues("managed", ResultServed).Inc()
	m.LayoutOrigin.WithLabelValues("api").Set(1)
	return registry
}

// TestTheTextfileIsThereWhileTheAgentRunsAndGoneAfter is the file's whole lifecycle: written
// at once rather than after the first interval, so a node exporter scraping right after a
// restart sees the agent, and removed on a clean stop, so that it does not go on reporting a
// layout origin that no longer describes anything.
func TestTheTextfileIsThereWhileTheAgentRunsAndGoneAfter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry-agent.prom")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Textfile{Log: quietLog(), Path: path, Gatherer: recorded(t), Interval: time.Hour}).Run(ctx)
	}()

	require.Eventually(t, func() bool {
		content, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(content), `d8_registry_agent_layout_origin{origin="api"} 1`)
	}, 5*time.Second, 10*time.Millisecond, "the file is written before the first tick")

	cancel()
	<-done

	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist, "a stopped agent leaves no series behind")

	// Nothing beside it either: the temporary file a write goes through is renamed or removed.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestStoppingTheTextfileReturnsOnlyOnceTheFileIsGone pins what the agent's exit relies on:
// when the function Start returned comes back, the file is already removed, with no waiting
// on anything else. The agent defers it and exits right after, so a removal still in flight
// at that moment would be one the process never gets to.
func TestStoppingTheTextfileReturnsOnlyOnceTheFileIsGone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry-agent.prom")

	stop := (&Textfile{Log: quietLog(), Path: path, Gatherer: recorded(t), Interval: time.Hour}).Start(context.Background())

	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "the file is written once started")

	stop()

	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist, "the file is still there when stop has returned")
}

// TestATextfileThatCannotBeWrittenIsRetried covers the directory not being there yet, or not
// being writable for a while. The error is logged and the next tick tries again: the loop is
// never what stops, because every pull on the node goes through the process running it.
func TestATextfileThatCannotBeWrittenIsRetried(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-yet")
	path := filepath.Join(dir, "registry-agent.prom")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Textfile{Log: quietLog(), Path: path, Gatherer: recorded(t), Interval: 10 * time.Millisecond}).Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("a failed write stopped the loop")
	default:
	}

	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "the write succeeds once the directory is there")

	cancel()
	<-done
}

// TestTheTextfileIsReadableByTheNodeExporter pins the mode: the node exporter does not run as
// root, and a file it cannot open is a scrape error on the node rather than missing series.
func TestTheTextfileIsReadableByTheNodeExporter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry-agent.prom")

	require.NoError(t, (&Textfile{Log: quietLog(), Path: path, Gatherer: recorded(t)}).Write())

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
