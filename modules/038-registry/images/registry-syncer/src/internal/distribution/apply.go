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
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Restarter makes the registry process pick up a configuration it has already
// read.
//
// The registry reads its configuration once at startup, so a change means the
// process has to come back. Doing that by signalling the peer container — the pod
// spec is never touched — is what keeps a credential change working while the
// Deckhouse operator is down. The same trick the module's existing nginx reloader
// uses.
type Restarter interface {
	Restart() error
}

// Applier keeps the rendered configuration on disk in step with the desired
// state, and restarts the registry when it actually changed.
type Applier struct {
	// ConfigPath is the configuration file the registry reads.
	ConfigPath string

	// UpstreamCAPath is where the upstream certificate authority is written. The
	// configuration only names the file, so the two must be written together or the
	// registry would start with a path that does not exist.
	UpstreamCAPath string

	// Restarter is invoked only when something changed.
	Restarter Restarter

	// Options are the pod-side parts of the rendering.
	Options Options
}

// Apply renders the configuration, writes it if it differs, and restarts the
// registry in that case. It reports whether anything changed.
//
// Change detection is on content, not on the resource version: a reconciliation
// loop re-renders constantly, and restarting the registry on every pass would
// make the storage permanently unavailable rather than merely reconfigured.
func (a *Applier) Apply(spec *registryv1alpha1.RegistryStorageSpec) (bool, error) {
	return a.apply(spec, a.Options)
}

// ApplyReadOnly configures the registry to refuse writes, and reports whether that changed
// anything.
//
// Separate from Apply rather than a field on the spec, because it is not part of the desired
// state: it is a temporary condition a replica puts itself in to collect garbage safely, and
// the next ordinary Apply must take it back out. Writing it into the spec would make a
// crashed collection permanent.
func (a *Applier) ApplyReadOnly(spec *registryv1alpha1.RegistryStorageSpec) (bool, error) {
	options := a.Options
	options.ReadOnly = true
	return a.apply(spec, options)
}

func (a *Applier) apply(spec *registryv1alpha1.RegistryStorageSpec, options Options) (bool, error) {
	rendered, err := Render(spec, options)
	if err != nil {
		return false, err
	}

	caChanged, err := a.writeUpstreamCA(spec)
	if err != nil {
		return false, err
	}

	configChanged, err := writeIfChanged(a.ConfigPath, rendered, 0o600)
	if err != nil {
		return false, fmt.Errorf("writing the configuration: %w", err)
	}

	if !configChanged && !caChanged {
		return false, nil
	}

	if a.Restarter != nil {
		if err := a.Restarter.Restart(); err != nil {
			// The configuration on disk is already the new one, so a failed restart
			// is recoverable: the next pass retries, and a crash of the registry
			// container picks it up anyway.
			return true, fmt.Errorf("restarting the registry: %w", err)
		}
	}
	return true, nil
}

// writeUpstreamCA keeps the upstream certificate authority file in step. An
// upstream without one leaves no stale file behind, since a leftover authority
// would keep being trusted after it was removed from the configuration.
func (a *Applier) writeUpstreamCA(spec *registryv1alpha1.RegistryStorageSpec) (bool, error) {
	if a.UpstreamCAPath == "" {
		return false, nil
	}

	certificate := ""
	if spec.Upstream != nil {
		certificate = spec.Upstream.CA
	}

	if certificate == "" {
		if err := os.Remove(a.UpstreamCAPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("removing the stale upstream certificate authority: %w", err)
		}
		return true, nil
	}

	changed, err := writeIfChanged(a.UpstreamCAPath, []byte(certificate), 0o644)
	if err != nil {
		return false, fmt.Errorf("writing the upstream certificate authority: %w", err)
	}
	return changed, nil
}

// writeIfChanged writes content atomically, and only when it differs from what is
// already there.
//
// Atomically because the registry may start at any moment, including halfway
// through a write, and a truncated configuration is a registry that does not come
// up at all.
func writeIfChanged(path string, content []byte, mode os.FileMode) (bool, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(existing, content):
		return false, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("creating the directory of %s: %w", path, err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return false, fmt.Errorf("creating a temporary file next to %s: %w", path, err)
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
	if err := os.Chmod(temporary.Name(), mode); err != nil {
		return false, fmt.Errorf("setting the mode of %s: %w", temporary.Name(), err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return false, fmt.Errorf("moving %s into place: %w", temporary.Name(), err)
	}

	return true, nil
}

// decodeBasic splits a base64-encoded "username:password" pair.
func decodeBasic(encoded string) (string, string, bool) {
	if encoded == "" {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}
	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", "", false
	}
	return username, password, true
}
