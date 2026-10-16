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
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// awaitPoll is how often the configuration is looked for while it is missing.
const awaitPoll = time.Second

// errStopped reports a stop asked for while waiting: the pod going away, not a failure.
var errStopped = errors.New("stopped while waiting for the configuration")

// awaitConfig waits until every path exists.
//
// The configuration is written by the syncer beside this container, and on a fresh pod this process
// starts first. Exiting on a missing file, as it did, is a crash to the kubelet: a few of them while
// the syncer waits for the API server, and the container sits in back-off for minutes after the
// configuration has arrived — measured on three masters, a replica not listening for five minutes on
// an empty store, its copy postponed pass after pass.
//
// Two signals, two meanings, and neither depends on what is on disk. SIGHUP is the syncer's news that
// the configuration is written: the syncer sends it, and only it, when the files were missing before
// its pass — which is exactly when this process is waiting. SIGTERM and SIGINT are a stop, always.
// They used to be both: a SIGTERM with the configuration written was read as the news, so the
// kubelet's SIGTERM on a pod being deleted was swallowed whenever the syncer had got there first, and
// the pod lived out its grace period.
//
// On return SIGHUP stays ignored for the life of the process, and SIGTERM and SIGINT get their default
// back — what the serving registry has always had, and what a restart by the syncer relies on. Ignored
// rather than reset, because the news can arrive after it is needed: the poll finds the files a moment
// before the syncer's signal, this returns, and a SIGHUP with its default action — like the SIGTERM
// the syncer sent before — would kill a registry that had just started on exactly that configuration,
// costing a container restart and the kubelet's back-off. A running registry has nothing to learn from
// it: a change to a configuration it already serves comes as SIGTERM, and is a restart.
func awaitConfig(log *slog.Logger, paths ...string) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	defer func() {
		// Ignore replaces the handler Notify installed, so there is no moment with SIGHUP at its
		// default between the two.
		signal.Ignore(syscall.SIGHUP)
		signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	}()

	return awaitSignalled(log, signals, awaitPoll, nil, paths...)
}

// awaitSignalled is the wait itself, with the signals and the poll given rather than taken from the
// process: a test that signals its own binary races signal.Notify, and loses to the default action.
//
// handled, when set, is told each time a signal has been dealt with and the wait goes on — for a test,
// which otherwise cannot tell a signal received from a signal looked into: the look at the disk comes
// after the receipt, and a file the test writes in between turned its probe into the news.
func awaitSignalled(
	log *slog.Logger, signals <-chan os.Signal, poll time.Duration, handled func(), paths ...string,
) error {
	if allExist(paths) {
		return nil
	}

	log.Info("waiting for the configuration the syncer writes", "paths", paths)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case received := <-signals:
			if received != syscall.SIGHUP {
				log.Info("stopping before the registry started", "signal", received.String())
				return errStopped
			}
			if !allExist(paths) {
				// Not expected: the syncer writes both files before it signals. Looking again on
				// the next tick costs nothing, and stopping here would be the crash this wait
				// exists to avoid.
				log.Warn("woken, but the configuration is still incomplete; waiting on", "paths", paths)
				if handled != nil {
					handled()
				}
				continue
			}
		}
		if allExist(paths) {
			log.Info("the configuration is written, starting")
			return nil
		}
	}
}

func allExist(paths []string) bool {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}
