package etcd

import (
	"path/filepath"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
)

type EtcdConfig struct {
	ManifestDir     string
	CertificatesDir string
}

type EtcdOption func(*EtcdConfig)

// WithManifestDir is an option to set the manifest directory.
func WithManifestDir(manifestDir string) EtcdOption {
	return func(o *EtcdConfig) {
		o.ManifestDir = manifestDir
	}
}

// WithCertificatesDir is an option to set the certificates directory.
func WithCertificatesDir(certificatesDir string) EtcdOption {
	return func(o *EtcdConfig) {
		o.CertificatesDir = certificatesDir
	}
}

func PrepareConfig(opts ...EtcdOption) *EtcdConfig {
	config := &EtcdConfig{}

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
