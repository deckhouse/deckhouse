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

// Package layout keeps the node's registry layout available, with or without a
// working API server.
//
// The agent is the thing that lets a node pull images, including the images of the
// control plane itself. So it cannot depend on the control plane being up: a cluster
// whose API server is down has to be recoverable by starting pods, and those pods
// need images.
//
// Hence the on-disk copy. It carries the credentials, because routing a pull means
// authenticating to a registry and there is nothing to dereference when the API is
// gone. That is the reason the layout object is flat and self-contained, and the
// reason this file is readable by nobody but the agent — unlike the runtime's own
// configuration, which containerd must be able to read.
package layout

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// DefaultCachePath is where the copy lives.
//
// Under /var rather than /etc: it is state the agent maintains, not configuration
// anyone edits, and on an immutable operating system /etc may not be writable at all.
const DefaultCachePath = "/var/lib/deckhouse/registry-agent/layout.json"

// cacheMode keeps the copy readable by its owner only. It holds registry
// credentials, and unlike the runtime configuration nothing else has to read it.
const cacheMode = 0o600

// Stored is what the copy holds: the layout and the generation it came from.
//
// The generation travels with it so the agent can still say which layout it applied
// after a restart during an outage. Without it the status would claim generation zero
// and an operator could not tell a caught-up agent from a stuck one.
type Stored struct {
	Generation int64                              `json:"generation"`
	Spec       *registryv1alpha1.RegistryNodeSpec `json:"spec"`
}

// Cache is the on-disk copy of the node layout.
type Cache struct {
	// Path of the copy. Empty means DefaultCachePath.
	Path string
}

func (c *Cache) path() string {
	if c.Path == "" {
		return DefaultCachePath
	}
	return c.Path
}

// Save writes the layout, atomically and privately.
//
// Atomically because the agent may be killed mid-write, and a truncated copy would
// leave the node unable to pull with no way to find out why — the failure would
// surface as an agent that starts and refuses to route.
func (c *Cache) Save(stored Stored) error {
	if stored.Spec == nil {
		return fmt.Errorf("no layout to save")
	}

	content, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encoding the layout: %w", err)
	}

	path := c.path()
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(existing, content):
		return nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("reading %s: %w", path, err)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating a temporary file in %s: %w", directory, err)
	}
	defer func() {
		_ = os.Remove(temporary.Name())
	}()

	// Narrowed before anything is written: CreateTemp is already 0600, but the file
	// carries credentials and relying on a default for that is not worth it.
	if err := temporary.Chmod(cacheMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("setting the mode of %s: %w", temporary.Name(), err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", temporary.Name(), err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("syncing %s: %w", temporary.Name(), err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", temporary.Name(), err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("moving %s into place: %w", temporary.Name(), err)
	}

	return nil
}

// Load reads the copy. A missing copy is not an error: a node that has never reached
// the API has nothing to fall back on, and saying so plainly is better than an error
// the caller has to classify.
func (c *Cache) Load() (*Stored, error) {
	content, err := os.ReadFile(c.path())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", c.path(), err)
	}

	stored := &Stored{}
	if err := json.Unmarshal(content, stored); err != nil {
		// A corrupt copy is worse than none: applying half a layout could point the
		// runtime at a backend that is not there.
		return nil, fmt.Errorf("decoding %s: %w", c.path(), err)
	}
	if stored.Spec == nil {
		return nil, fmt.Errorf("%s holds no layout", c.path())
	}
	return stored, nil
}
