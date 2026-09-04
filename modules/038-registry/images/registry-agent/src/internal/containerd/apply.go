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

package containerd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Writer converges the drop-in directory on the desired content.
type Writer struct {
	// Root is the drop-in directory.
	Root string
}

// Result is what an apply changed, so the caller can log something useful and
// report a meaningful status instead of "reconciled".
type Result struct {
	// Written are the files whose content changed.
	Written []string

	// RemovedHosts are the host directories taken away, each of which means a
	// registry the node can no longer reach.
	RemovedHosts []string
}

// Changed reports whether anything was actually modified.
func (r *Result) Changed() bool {
	return len(r.Written) > 0 || len(r.RemovedHosts) > 0
}

// Apply writes what differs and removes what the agent previously created and no
// longer wants.
//
// Only the agent's own leftovers are removed, tracked in a state file. A host
// directory the agent never created is left alone: the directory is shared with
// whatever else an administrator may have put there, and deleting a stranger's
// configuration would be a worse failure than leaving one behind.
func (w *Writer) Apply(desired Desired) (Result, error) {
	result := Result{}

	root := w.Root
	if root == "" {
		root = DefaultRoot
	}

	previous, err := w.readState(root)
	if err != nil {
		return result, err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return result, fmt.Errorf("creating %s: %w", root, err)
	}

	paths := make([]string, 0, len(desired.Files))
	for path := range desired.Files {
		paths = append(paths, path)
	}
	// Sorted so a change set is reproducible and a log line about it means the same
	// thing every time.
	sort.Strings(paths)

	for _, path := range paths {
		changed, err := writeIfChanged(filepath.Join(root, path), desired.Files[path])
		if err != nil {
			return result, err
		}
		if changed {
			result.Written = append(result.Written, path)
		}
	}

	for _, host := range previous {
		if slices.Contains(desired.Hosts, host) {
			continue
		}
		// A host that is configured no longer: its directory has to go, or the runtime
		// would keep reaching a registry that was removed — with credentials that may
		// have been revoked.
		if err := os.RemoveAll(filepath.Join(root, host)); err != nil {
			return result, fmt.Errorf("removing the configuration of %s: %w", host, err)
		}
		result.RemovedHosts = append(result.RemovedHosts, host)
	}

	if err := w.writeState(root, desired.Hosts); err != nil {
		return result, err
	}

	return result, nil
}

// readState lists the hosts the agent created directories for.
//
// An unreadable or corrupt state file is treated as "nothing known", which errs
// towards leaving directories behind rather than deleting configuration on a
// parsing error.
func (w *Writer) readState(root string) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(root, StateFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", StateFile, err)
	}

	var hosts []string
	if err := json.Unmarshal(content, &hosts); err != nil {
		return nil, nil
	}
	return hosts, nil
}

func (w *Writer) writeState(root string, hosts []string) error {
	if hosts == nil {
		hosts = []string{}
	}

	content, err := json.Marshal(hosts)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", StateFile, err)
	}

	if _, err := writeIfChanged(filepath.Join(root, StateFile), content); err != nil {
		return err
	}
	return nil
}

// writeIfChanged writes content atomically, and only when it differs.
//
// Atomically because containerd reads this directory on every pull, including in the
// middle of a write, and a half-written configuration is a pull that fails for a
// reason nobody will connect to a configuration change.
//
// Only when it differs because the agent re-renders on a schedule, and rewriting
// identical files would keep the modification times moving for no reason and defeat
// any change detection above.
func writeIfChanged(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(existing, content):
		return false, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return false, fmt.Errorf("creating %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return false, fmt.Errorf("creating a temporary file in %s: %w", directory, err)
	}
	defer func() {
		_ = os.Remove(temporary.Name())
	}()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return false, fmt.Errorf("writing %s: %w", temporary.Name(), err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return false, fmt.Errorf("syncing %s: %w", temporary.Name(), err)
	}
	if err := temporary.Close(); err != nil {
		return false, fmt.Errorf("closing %s: %w", temporary.Name(), err)
	}
	// World-readable: containerd reads these, and it does not run as this process.
	// The credentials inside are no more exposed than they already are in the
	// runtime's own memory, and a directory the runtime cannot read is a node that
	// cannot pull.
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		return false, fmt.Errorf("setting the mode of %s: %w", temporary.Name(), err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return false, fmt.Errorf("moving %s into place: %w", temporary.Name(), err)
	}

	return true, nil
}

// Writable reports whether the drop-in directory can be written to, and why not
// when it cannot.
//
// Checked explicitly because an immutable operating system with a read-only root
// filesystem is a supported target, and there the agent cannot own this directory at
// all. Failing with that stated is much better than failing on the first write with
// a bare permission error, which is what the existing bashible steps do.
func (w *Writer) Writable() error {
	root := w.Root
	if root == "" {
		root = DefaultRoot
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("%s cannot be created, which an immutable root filesystem looks like: %w", root, err)
	}

	probe, err := os.CreateTemp(root, ".writable.*")
	if err != nil {
		return fmt.Errorf("%s is not writable, which an immutable root filesystem looks like: %w", root, err)
	}
	name := probe.Name()
	_ = probe.Close()

	if err := os.Remove(name); err != nil {
		return fmt.Errorf("cannot clean up in %s: %w", root, err)
	}
	return nil
}

// HostsOf lists the host directories currently present, excluding the state file.
// Used for reporting what the node is configured for.
func (w *Writer) HostsOf() ([]string, error) {
	root := w.Root
	if root == "" {
		root = DefaultRoot
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}

	hosts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			hosts = append(hosts, entry.Name())
		}
	}
	sort.Strings(hosts)
	return hosts, nil
}
