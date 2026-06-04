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

package etcd

import (
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
)

type options struct {
	ManifestDir     string
	CertificatesDir string
}

type option func(*options)

// WithManifestDir is an option to set the manifest directory.
func WithManifestDir(manifestDir string) option {
	return func(o *options) {
		o.ManifestDir = manifestDir
	}
}

// WithCertificatesDir is an option to set the certificates directory.
func WithCertificatesDir(certificatesDir string) option {
	return func(o *options) {
		o.CertificatesDir = certificatesDir
	}
}

func prepareOptions(opts ...option) *options {
	config := &options{}

	for _, option := range opts {
		option(config)
	}

	if config.ManifestDir == "" {
		config.ManifestDir = filepath.Join(constants.KubernetesDir, "manifests")
	}

	if config.CertificatesDir == "" {
		config.CertificatesDir = constants.KubernetesDir
	}

	return config
}
