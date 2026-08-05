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

package distribution

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// SignalRestarter makes the registry come back by signalling its process, which
// the kubelet then restarts.
//
// The registry reads its configuration once at startup, so a change means the
// process has to come back. Signalling the peer container rather than editing the
// pod is deliberate: the pod object is never touched, which is what keeps a
// credential change working while the Deckhouse operator is down. The pod shares a
// process namespace for this, the same way the module's nginx reloader already
// works.
//
// The cost is a brief gap on this replica. That is why the storage runs as several
// replicas, and why a configuration change is applied to each independently rather
// than to all at once.
type SignalRestarter struct {
	// ProcessName is the executable to look for, matched on the base name of the
	// command line.
	ProcessName string

	// ProcDir is the process filesystem to search, overridable for tests.
	ProcDir string

	// Signal is sent to the process. SIGTERM lets it finish in-flight requests.
	Signal syscall.Signal

	// signal is the injection point for tests; nil means the real system call.
	signal func(pid int, sig syscall.Signal) error
}

var _ Restarter = (*SignalRestarter)(nil)

// Restart signals every matching process.
func (s *SignalRestarter) Restart() error {
	procDir := s.ProcDir
	if procDir == "" {
		procDir = "/proc"
	}
	signalNumber := s.Signal
	if signalNumber == 0 {
		signalNumber = syscall.SIGTERM
	}

	pids, err := s.find(procDir)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		// Not an error worth failing the pass over: the registry may be starting, or
		// may have just crashed, and either way it will read the new configuration
		// when it comes up.
		return fmt.Errorf("no process named %q found in %s", s.ProcessName, procDir)
	}

	send := s.signal
	if send == nil {
		send = syscall.Kill
	}

	for _, pid := range pids {
		if err := send(pid, signalNumber); err != nil {
			return fmt.Errorf("signalling process %d: %w", pid, err)
		}
	}
	return nil
}

// find lists the process identifiers whose command line names the wanted
// executable. Own process excluded, so a syncer named like the registry cannot
// signal itself into a restart loop.
func (s *SignalRestarter) find(procDir string) ([]int, error) {
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", procDir, err)
	}

	self := os.Getpid()
	var pids []int

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}

		content, err := os.ReadFile(filepath.Join(procDir, entry.Name(), "cmdline"))
		if err != nil {
			// The process exited between the listing and now, which is ordinary.
			continue
		}

		if matchesProcess(content, s.ProcessName) {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// matchesProcess compares the wanted name against the executable of a command
// line, which is NUL-separated in the process filesystem.
func matchesProcess(cmdline []byte, want string) bool {
	if want == "" {
		return false
	}

	fields := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
	if len(fields) == 0 || fields[0] == "" {
		return false
	}

	// Only the executable is matched, never the arguments: a configuration path
	// mentioning the name would otherwise match anything that reads it, including
	// the syncer itself.
	return filepath.Base(fields[0]) == want
}
