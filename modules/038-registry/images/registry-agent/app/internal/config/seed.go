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

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"sigs.k8s.io/yaml"
)

// SeedMirror is the optional on-node bootstrap seed the agent appends as a
// lower-priority containerd mirror on the primary host during the air-gap install
// window, until the in-cluster cache is filled and the seed is torn down.
type SeedMirror struct {
	// URL is the full endpoint, e.g. "https://127.0.0.1:5010".
	URL string
	// CA is the PEM CA bundle for the seed endpoint (module CA).
	CA string
}

// seedFile is the YAML shape of /etc/kubernetes/registry-agent-bootstrap/bootstrap-seed.yaml.
type seedFile struct {
	Host   string `json:"host"`
	Scheme string `json:"scheme"`
	CA     string `json:"ca"`
}

// LoadSeedMirror reads the optional bootstrap-seed file. An empty path or an
// absent file yields (nil, nil) — the seed is optional. A present-but-unparseable
// or host-less file is an error / nil so the agent renders no broken mirror.
func LoadSeedMirror(path string) (*SeedMirror, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read bootstrap-seed file %q: %w", path, err)
	}
	var f seedFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse bootstrap-seed file %q: %w", path, err)
	}
	if f.Host == "" {
		return nil, nil
	}
	scheme := f.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return &SeedMirror{URL: scheme + "://" + f.Host, CA: f.CA}, nil
}
