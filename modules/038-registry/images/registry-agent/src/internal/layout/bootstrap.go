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

package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// DefaultBootstrapPath is where bashible writes the layout a node starts with.
//
// Next to the agent's certificate material rather than under /var with the cache,
// because it is the same kind of thing: configuration placed on the node by whoever
// installed the agent, not state the agent maintains.
const DefaultBootstrapPath = "/etc/kubernetes/registry-agent/bootstrap-layout.json"

// Bootstrap is the layout a node uses before it has ever reached the API server.
//
// It exists because of an ordering that has no other way out. The agent is on the path
// of every pull on the node, which includes the pulls that bring up the control plane,
// so a node joining a new cluster has to be able to route before there is an API server
// to ask. The layout it needs then is also the simplest one — reach the upstream
// directly — and it is known at install time, so bashible writes it down.
//
// Only ever a fallback. The moment the API server answers, what it says wins and is
// stored in the cache, and the cache outranks this seed from then on: this is what the
// node was installed with, not what the cluster currently wants.
//
// Two places it can come from, tried in that order, because two kinds of node put it
// there differently. A bashible node gets a file written by the step that installs the
// agent. An Engine node gets none — /etc/kubernetes there is a tmpfs this agent writes,
// and the object that starts it carries a pod manifest and nothing else — so the seed is
// taken from the node's own NodeConfig, which already holds the same credentials in a
// file only root reads. See nodeConfigSeed.
type Bootstrap struct {
	// Path of the file. Empty means DefaultBootstrapPath.
	Path string

	// NodeConfigPath is the node's own configuration, read when there is no file at
	// Path. Empty means nodeconfig.DefaultPath.
	NodeConfigPath string
}

func (b *Bootstrap) path() string {
	if b == nil || b.Path == "" {
		return DefaultBootstrapPath
	}
	return b.Path
}

// Load reads the layout the node was installed with, returning nothing when this node
// has none.
//
// Nothing is normal, in both shapes: a node installed into an existing cluster has an API
// server to ask from the start, and one whose config names no registry of its own is told
// where to pull from by the cluster alone.
func (b *Bootstrap) Load() (*registryv1alpha1.RegistryNodeSpec, error) {
	if b == nil {
		return nil, nil
	}

	spec, err := b.fromFile()
	if err != nil || spec != nil {
		return spec, err
	}

	// The file wins when both exist. On a node that has one it is the more specific
	// answer: it was written for this agent by whoever installed it, whereas the node
	// config describes how the node itself reaches a registry.
	return nodeConfigSeed{Path: b.NodeConfigPath}.load()
}

// fromFile reads the seed a bashible step wrote, or nothing when there is no such file.
func (b *Bootstrap) fromFile() (*registryv1alpha1.RegistryNodeSpec, error) {
	content, err := os.ReadFile(b.path())
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading the bootstrap layout %s: %w", b.path(), err)
	}

	if len(content) == 0 {
		return nil, nil
	}

	// A plain RegistryNodeSpec, not the cache's envelope: the cache carries a
	// generation because it is a copy of a specific object version, whereas this file
	// belongs to no generation at all — it is what the node was installed with.
	spec := &registryv1alpha1.RegistryNodeSpec{}
	if err := json.Unmarshal(content, spec); err != nil {
		// Reported rather than ignored. Applying half a layout would point the runtime
		// at a backend that is not there, and the resulting pull failures would say
		// nothing about a malformed file.
		return nil, fmt.Errorf("the bootstrap layout %s is unusable: %w", b.path(), err)
	}
	if len(spec.Backends) == 0 {
		// Nothing to route to, so nothing to be gained by pointing the runtime at the
		// agent; better to say so than to serve a layout that cannot answer.
		return nil, fmt.Errorf("the bootstrap layout %s names no backend", b.path())
	}

	return spec, nil
}
