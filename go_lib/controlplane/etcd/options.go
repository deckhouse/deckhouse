package etcd

import (
	"path/filepath"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
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
