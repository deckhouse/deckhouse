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

// Package nodeconfig reads the document an Engine node brings itself up from.
//
// The agent needs two things from it, and on such a node there is nowhere else to get
// either: the registry the node was installed from, which is the layout it starts on
// before the API server has ever answered, and the API server's address, which is how it
// reaches the API server in the first place. Both are there because the node's own agent
// needs them for the same reasons.
//
// Read rather than received. The object that puts this agent on an Engine node carries a
// pod manifest and nothing else — deliberately, since a manifest is world-readable and
// these are credentials — so what it needs beyond the manifest it finds already on the
// node, in a file only root reads.
package nodeconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"sigs.k8s.io/yaml"
)

// DefaultPath is where nodelet keeps the document.
//
// Source of truth: config.DefaultPath in the nodelet repository. A contract with another
// repository, so it is pinned by a test rather than only written down.
const DefaultPath = "/config/nodeconfig.yaml"

// Document is the part of a NodeConfig this reads.
//
// A local copy of two fields rather than an import of nodelet's types: the agent has no
// dependency on that repository and should not grow one for this. The field names are the
// contract, and a change to them shows up as a node with no seed and no API address —
// which is a warning and a retry, not a node that fails to start.
type Document struct {
	Spec Spec `json:"spec"`
}

// Spec is NodeConfig's spec, trimmed to what the agent asks of it.
type Spec struct {
	// APIServerEndpoints are where the API server answers. Either spelling: a config
	// written by node-controller carries the full "https://host:port", and nodelet's own
	// ParseEndpoint also accepts a bare "host:port". See apiServerURL.
	APIServerEndpoints []string `json:"apiServerEndpoints"`

	// Registry is direct registry access: how the node pulls its system extensions and
	// control-plane images without anything in the cluster helping it.
	Registry *Registry `json:"registry"`

	// Kubelet is the node's kubelet settings, of which one field is read here.
	Kubelet Kubelet `json:"kubelet"`
}

// Kubelet is NodeConfig's spec.kubelet, trimmed to the cluster authority.
type Kubelet struct {
	// CACert is the base64-encoded cluster CA, the one the kubelet verifies the API
	// server with. Read from here rather than from /etc/kubernetes/pki/ca.crt so that
	// the agent needs no mount of that directory — on a master it also holds the
	// cluster CA's private key, and one file of it cannot be mounted without the pod
	// hanging on a node where the file is not there yet.
	CACert string `json:"caCert"`
}

// Registry is NodeConfig's spec.registry.
type Registry struct {
	Address string `json:"address"`
	Path    string `json:"path"`
	Scheme  string `json:"scheme"`
	CA      string `json:"ca"`
	// Auth is base64("user:password"), the form a docker config keeps it in.
	Auth string `json:"auth"`
}

// Load reads the document, returning nothing when this node has none.
//
// Nothing is the ordinary answer on a bashible node, which is brought up by a different
// mechanism entirely and has no such file.
func Load(path string) (*Document, error) {
	if path == "" {
		path = DefaultPath
	}

	content, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading the node config %s: %w", path, err)
	}

	var document Document
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("the node config %s is unusable: %w", path, err)
	}
	return &document, nil
}
