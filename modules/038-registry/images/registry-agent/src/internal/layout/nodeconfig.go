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
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-agent/internal/nodeconfig"
)

// nodeConfigSeed is the layout an Engine node starts with, taken from its own config.
//
// It exists because the file a bashible node has is not available on an Engine one, and
// nothing else can put a private file there. /etc/kubernetes is a tmpfs this very agent
// writes, and the manifest that starts the agent is world-readable, so a seed carrying
// registry credentials cannot travel that way. The node's own NodeConfig already holds
// exactly those credentials, in a file only root reads, put there by whoever installed
// the node — so the seed is read from where it already is rather than delivered again.
//
// What it names is the registry the node was installed from, not the cluster's in-cluster
// store: the seed's whole job is to get the node as far as the API server, which then says
// where images really come from. `Cache` is therefore false — claiming the store here would
// point the runtime at something that may not exist yet.
//
// Absent is not an error. An ordinary bashible node has no such file, and a node joining an
// existing cluster has an API server to ask from the start.
type nodeConfigSeed struct {
	// Path of the document. Empty means nodeconfig.DefaultPath.
	Path string
}

// load returns the seed, or nothing when this node has no config of its own.
func (n nodeConfigSeed) load() (*registryv1alpha1.RegistryNodeSpec, error) {
	document, err := nodeconfig.Load(n.Path)
	if err != nil || document == nil {
		return nil, err
	}

	registry := document.Spec.Registry
	if registry == nil || registry.Address == "" {
		// A node configured to reach no registry directly. Nothing to seed from, and
		// nothing wrong either: it will route from what the API server tells it.
		return nil, nil
	}

	scheme := registryv1alpha1.Scheme(registry.Scheme)
	if scheme == "" {
		// The CRD default, applied here because a document read from a file passes no
		// API server. Same reasoning as nodelet's own loader.
		scheme = registryv1alpha1.SchemeHTTPS
	}

	endpoint := registryv1alpha1.Endpoint{
		Scheme: scheme,
		Host:   registry.Address,
		Path:   registry.Path,
		CA:     registry.CA,
	}
	if registry.Auth != "" {
		// base64("user:password") is the form both sides keep it in, so it is copied
		// rather than parsed.
		endpoint.Auth = &registryv1alpha1.Auth{Auth: registry.Auth}
	}

	return &registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendUpstream,
			Endpoint: endpoint,
		}},
	}, nil
}
