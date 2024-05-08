/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultDirMode  = os.FileMode(0755)
	DefaultFileMode = os.FileMode(0755)
	DefaultCertMode = os.FileMode(644)
)

var (
	TmpWorkspaceDir = filepath.Join(os.TempDir(), "system-registry-manager/workspace")

	TmpWorkspaceCertsDir                  = filepath.Join(TmpWorkspaceDir, "pki")
	TmpWorkspaceManifestsDir              = filepath.Join(TmpWorkspaceDir, "manifests")
	TmpWorkspaceStaticPodsDir             = filepath.Join(TmpWorkspaceManifestsDir, "static-pods")
	TmpWorkspaceSeaweedManifestsDir       = filepath.Join(TmpWorkspaceManifestsDir, "seaweedfs")
	TmpWorkspaceDockerAuthManifestsDir    = filepath.Join(TmpWorkspaceManifestsDir, "docker-auth")
	TmpWorkspaceDockerDistribManifestsDir = filepath.Join(TmpWorkspaceManifestsDir, "distribution")

	InputCertsDir                  = "/pki"
	InputManifestsDir              = "/manifests"
	InputStaticPodsDir             = filepath.Join(InputManifestsDir, "static-pods")
	InputSeaweedManifestsDir       = filepath.Join(InputManifestsDir, "seaweedfs")
	InputDockerAuthManifestsDir    = filepath.Join(InputManifestsDir, "docker-auth")
	InputDockerDistribManifestsDir = filepath.Join(InputManifestsDir, "distribution")

	DestionationDir              = "/etc/kubernetes"
	DestinationSystemRegistryDir = filepath.Join(DestionationDir, "system-registry")
	DestionationDirStaticPodsDir = filepath.Join(DestionationDir, "manifests")

	DestionationSeaweedManifestsDir       = filepath.Join(DestinationSystemRegistryDir, "seaweedfs")
	DestionationDockerAuthManifestsDir    = filepath.Join(DestinationSystemRegistryDir, "docker-auth")
	DestionationDockerDistribManifestsDir = filepath.Join(DestinationSystemRegistryDir, "distribution")
)

type ManifestSpec struct {
	InputPath string
	TmpPath   string
	DestPath  string
}

type CaCertificateSpec struct {
	InputPath string
	TmpPath   string
}

type CertificateSpec struct {
	TmpGeneratePath string
	DestPath        string
}

type BaseCertificatesSpec struct {
	CACrt     CaCertificateSpec
	CAKey     CaCertificateSpec
	EtcdCACrt CaCertificateSpec
	EtcdCAKey CaCertificateSpec
}

type GeneratedCertificateSpec struct {
	CAKey  CaCertificateSpec
	CACert CaCertificateSpec
	Key    CertificateSpec
	Cert   CertificateSpec
}

type GeneratedCertificatesSpec struct {
	SeaweedEtcdClientCert GeneratedCertificateSpec
	DockerAuthTokenCert   GeneratedCertificateSpec
}

type ManifestsSpec struct {
	// BaseCertificates      BaseCertificatesSpec
	GeneratedCertificates GeneratedCertificatesSpec
	Manifests             []ManifestSpec
}

func NewManifestsSpec() *ManifestsSpec {
	baseCertificates := BaseCertificatesSpec{
		CACrt: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "ca.crt"),
			TmpPath:   filepath.Join(TmpWorkspaceCertsDir, "ca.crt"),
		},
		CAKey: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "ca.key"),
			TmpPath:   filepath.Join(TmpWorkspaceCertsDir, "ca.key"),
		},
		EtcdCACrt: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "etcd-ca.crt"),
			TmpPath:   filepath.Join(TmpWorkspaceCertsDir, "etcd-ca.crt"),
		},
		EtcdCAKey: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "etcd-ca.key"),
			TmpPath:   filepath.Join(TmpWorkspaceCertsDir, "etcd-ca.key"),
		},
	}

	generatedCertificates := GeneratedCertificatesSpec{
		SeaweedEtcdClientCert: GeneratedCertificateSpec{
			CAKey:  baseCertificates.EtcdCAKey,
			CACert: baseCertificates.EtcdCACrt,
			Key: CertificateSpec{
				TmpGeneratePath: filepath.Join(DestionationSeaweedManifestsDir, "seaweedfs-etcd-client.key"),
				DestPath:        filepath.Join(DestionationSeaweedManifestsDir, "seaweedfs-etcd-client.key"),
			},
			Cert: CertificateSpec{
				TmpGeneratePath: filepath.Join(DestionationSeaweedManifestsDir, "seaweedfs-etcd-client.crt"),
				DestPath:        filepath.Join(DestionationSeaweedManifestsDir, "seaweedfs-etcd-client.crt"),
			},
		},
		DockerAuthTokenCert: GeneratedCertificateSpec{
			CAKey:  baseCertificates.EtcdCAKey,
			CACert: baseCertificates.EtcdCACrt,
			Key: CertificateSpec{
				TmpGeneratePath: filepath.Join(DestionationSeaweedManifestsDir, "token.key"),
				DestPath:        filepath.Join(DestionationDockerAuthManifestsDir, "token.key"),
			},
			Cert: CertificateSpec{
				TmpGeneratePath: filepath.Join(DestionationSeaweedManifestsDir, "token.crt"),
				DestPath:        filepath.Join(DestionationDockerAuthManifestsDir, "token.crt"),
			},
		},
	}

	manifestsSpec := ManifestsSpec{
		// BaseCertificates:      baseCertificates,
		GeneratedCertificates: generatedCertificates,
		Manifests: []ManifestSpec{
			{
				InputPath: filepath.Join(InputDockerDistribManifestsDir, "config.yaml"),
				TmpPath:   filepath.Join(TmpWorkspaceDockerDistribManifestsDir, "config.yaml"),
				DestPath:  filepath.Join(DestionationDockerDistribManifestsDir, "config.yaml"),
			},
			{
				InputPath: filepath.Join(InputDockerAuthManifestsDir, "config.yaml"),
				TmpPath:   filepath.Join(TmpWorkspaceDockerAuthManifestsDir, "config.yaml"),
				DestPath:  filepath.Join(DestionationDockerAuthManifestsDir, "config.yaml"),
			},
			{
				InputPath: filepath.Join(InputSeaweedManifestsDir, "filer.toml"),
				TmpPath:   filepath.Join(TmpWorkspaceSeaweedManifestsDir, "filer.toml"),
				DestPath:  filepath.Join(DestionationSeaweedManifestsDir, "filer.toml"),
			},
			{
				InputPath: filepath.Join(InputSeaweedManifestsDir, "master.toml"),
				TmpPath:   filepath.Join(TmpWorkspaceSeaweedManifestsDir, "master.toml"),
				DestPath:  filepath.Join(DestionationSeaweedManifestsDir, "master.toml"),
			},
			{
				InputPath: filepath.Join(InputStaticPodsDir, "system-registry.yaml"),
				TmpPath:   filepath.Join(TmpWorkspaceStaticPodsDir, "system-registry.yaml"),
				DestPath:  filepath.Join(DestionationDirStaticPodsDir, "system-registry.yaml"),
			},
		},
	}
	return &manifestsSpec
}
