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

package ondemand

import (
	"encoding/json"
	"os"
	"slices"
	"sync"

	"golang.org/x/mod/semver"
)

// EmbeddedVersions are the minor versions of Deckhouse (v1.NN) whose
// documentation of an embedded module the public site keeps in its bucket.
// They come from the history file the site copies from the bucket into a
// ConfigMap, {"latest": [modules], "v1.NN": [modules]}. The file is mounted
// with subPath and stays the same for the life of the pod: a change of the
// ConfigMap restarts the pod (pod-reloader.deckhouse.io/auto watches the
// ConfigMaps of its volumes). So it is read once it is read well; a failed
// read is tried again on the next request.
type EmbeddedVersions struct {
	path string

	mu       sync.Mutex
	byModule map[string][]string
}

// NewEmbeddedVersions lists the versions from the history file at path.
func NewEmbeddedVersions(path string) *EmbeddedVersions {
	return &EmbeddedVersions{path: path}
}

func (e *EmbeddedVersions) read() (map[string][]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.byModule == nil {
		byModule, err := readEmbeddedHistory(e.path)
		if err != nil {
			return nil, err
		}
		e.byModule = byModule
	}

	return e.byModule, nil
}

// List returns the versions of the module, the newest first; none for a
// module that was never embedded. The slice is shared: callers must not
// change it.
func (e *EmbeddedVersions) List(module string) ([]string, error) {
	byModule, err := e.read()
	if err != nil {
		return nil, err
	}

	return byModule[module], nil
}

func readEmbeddedHistory(path string) (map[string][]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var history map[string][]string
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, err
	}

	byModule := make(map[string][]string)
	for version, modules := range history {
		// Minor versions of 1 only, those nginx serves (v1\.[0-9]+): latest
		// is the main branch, served under its own address.
		if !semver.IsValid(version) || semver.Canonical(version) != version+".0" || semver.Major(version) != "v1" {
			continue
		}
		for _, module := range modules {
			byModule[module] = append(byModule[module], version)
		}
	}
	for module, versions := range byModule {
		slices.SortFunc(versions, func(a, b string) int { return semver.Compare(b, a) })
		// A module listed twice under one version.
		byModule[module] = slices.Compact(versions)
	}

	return byModule, nil
}
