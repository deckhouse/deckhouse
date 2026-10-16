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

package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// never is a poll that does not come within a test, so that only a signal can end the wait.
const never = time.Hour

// await runs the wait with its own signal channel. Nothing here signals the test binary: a signal
// sent before signal.Notify is reached kills the binary instead of reaching the wait.
func await(t *testing.T, poll time.Duration, paths ...string) (chan<- os.Signal, chan error) {
	t.Helper()
	signals := make(chan os.Signal)
	done := make(chan error, 1)
	handled := make(chan struct{}, 16)
	go func() {
		done <- awaitSignalled(quiet, signals, poll, func() { handled <- struct{}{} }, paths...)
	}()
	// Keyed as the callers hold it: a bidirectional channel is another key in an interface.
	probes.Store((chan<- os.Signal)(signals), handled)
	return signals, done
}

// probes are the wait's notices that a signal was dealt with, by its signal channel — see waiting.
var probes sync.Map

// send delivers a signal to a wait that is still receiving; a wait that has already returned fails
// the test rather than hanging it.
func send(t *testing.T, signals chan<- os.Signal, received os.Signal) {
	t.Helper()
	select {
	case signals <- received:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s was not received: the wait is not waiting", received)
	}
}

// waiting returns once the wait is past its first look and in its loop, so that a file written next
// is found by the loop and not by that first look. A SIGHUP with the configuration incomplete changes
// nothing, which is what makes it the probe — and it returns only once the wait has looked at the disk
// for it: returning on the receipt alone, a file written next could be found by the probe's own look,
// and the probe became the news.
func waiting(t *testing.T, signals chan<- os.Signal) {
	t.Helper()
	send(t, signals, syscall.SIGHUP)
	handled, _ := probes.Load(signals)
	select {
	case <-handled.(chan struct{}):
	case <-time.After(5 * time.Second):
		t.Fatal("the probe was received and never dealt with")
	}
}

func result(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting")
		return nil
	}
}

func notYet(t *testing.T, done chan error) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("returned before the configuration was written: %v", err)
	default:
	}
}

func write(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAwaitConfigReturnsAtOnceWhenItIsThere(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.yaml")
	write(t, config)
	if err := awaitSignalled(quiet, nil, never, nil, config); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// The fresh pod: the syncer writes the configuration after this process started. It waits rather
// than crashing into the kubelet's back-off, and finds the files on its own if no signal comes.
func TestAwaitConfigWaitsForTheSyncer(t *testing.T) {
	dir := t.TempDir()
	config, wrapper := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "deckhouse.yaml")
	_, done := await(t, 10*time.Millisecond, config, wrapper)

	notYet(t, done)
	write(t, config, wrapper)
	if err := result(t, done); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// The syncer's SIGHUP after writing is the news, and starts the registry without waiting for a poll.
func TestAwaitConfigTakesTheSyncersHangupAsTheNews(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.yaml")
	signals, done := await(t, never, config)

	waiting(t, signals)
	write(t, config)
	send(t, signals, syscall.SIGHUP)
	if err := result(t, done); err != nil {
		t.Fatalf("a SIGHUP with the configuration written must start the registry, got %v", err)
	}
}

// A SIGHUP that finds the configuration incomplete is not a stop: the wait goes on.
func TestAwaitConfigWaitsOnAfterAnEarlyHangup(t *testing.T) {
	dir := t.TempDir()
	config, wrapper := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "deckhouse.yaml")
	signals, done := await(t, never, config, wrapper)

	write(t, config)
	send(t, signals, syscall.SIGHUP)
	notYet(t, done)

	write(t, wrapper)
	send(t, signals, syscall.SIGHUP)
	if err := result(t, done); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// A SIGTERM with nothing written is the pod going away.
func TestAwaitConfigStopsOnATermWithNothingWritten(t *testing.T) {
	signals, done := await(t, never, filepath.Join(t.TempDir(), "config.yaml"))

	send(t, signals, syscall.SIGTERM)
	if err := result(t, done); !errors.Is(err, errStopped) {
		t.Fatalf("a SIGTERM before the configuration exists must stop it, got %v", err)
	}
}

// And so is a SIGTERM with the configuration already written: the kubelet's SIGTERM is a stop whoever
// got there first. Read as the news, it started a registry on a pod being deleted, which then lived
// out its grace period.
func TestAwaitConfigStopsOnATermWithTheConfigurationWritten(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.yaml")
	signals, done := await(t, never, config)

	waiting(t, signals)
	write(t, config)
	send(t, signals, syscall.SIGTERM)
	if err := result(t, done); !errors.Is(err, errStopped) {
		t.Fatalf("a SIGTERM must stop it whatever is on disk, got %v", err)
	}
}

func TestAwaitConfigStopsOnAnInterrupt(t *testing.T) {
	signals, done := await(t, never, filepath.Join(t.TempDir(), "config.yaml"))

	send(t, signals, syscall.SIGINT)
	if err := result(t, done); !errors.Is(err, errStopped) {
		t.Fatalf("a SIGINT must stop it, got %v", err)
	}
}

// The syncer's SIGHUP may arrive after the poll already found the files and the registry started.
// Its default action is to terminate, so a SIGHUP left at the default would kill a registry that had
// just started on exactly that configuration. Checked through the process's disposition rather than
// by sending one: see await.
func TestAwaitConfigLeavesTheHangupIgnored(t *testing.T) {
	t.Cleanup(func() { signal.Reset(syscall.SIGHUP) })

	config := filepath.Join(t.TempDir(), "config.yaml")
	write(t, config)
	if err := awaitConfig(quiet, config); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("SIGHUP must stay ignored once the registry starts")
	}
}
