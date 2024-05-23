/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/certificate"
	"os"
	"path/filepath"
	"system-registry-manager/pkg"
	"time"
)

const (
	DefaultDirMode  = os.FileMode(0755)
	DefaultFileMode = os.FileMode(0755)
	DefaultCertMode = os.FileMode(644)
)

var (
	SystemRegistryManagerLocation          = "/deckhouse/ee/modules/038-system-registry/images/system-registry-manager/"
	TmpDirForSystemRegistryManagerLocation = filepath.Join(SystemRegistryManagerLocation, "test_data")
)

func getTmpWorkspaceDir() string {
	if os.Getenv("IS_TEST") == "" {
		return filepath.Join(os.TempDir(), "system-registry-manager/workspace")
	}
	return filepath.Join(TmpDirForSystemRegistryManagerLocation, "workspace")
}

func getInputCertsDir() string {
	if os.Getenv("IS_TEST") == "" {
		return "/pki"
	}
	return filepath.Join(TmpDirForSystemRegistryManagerLocation, "pki")
}

func getInputManifestsDir() string {
	if os.Getenv("IS_TEST") == "" {
		return "/templates"
	}
	return filepath.Join(SystemRegistryManagerLocation, "templates")
}

func getDestionationDir() string {
	if os.Getenv("IS_TEST") == "" {
		return "/etc/kubernetes"
	}
	return filepath.Join(TmpDirForSystemRegistryManagerLocation, "etc_k&s_destionation")
}

type ManifestSpec struct {
	IsStaticPodManifest bool
	NeedChangeFileBy    pkg.NeedChangeFileBy
	InputPath           string
	TmpPath             string
	DestPath            string
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
	NeedChangeFileBy pkg.NeedChangeFileBy
	CAKey            CaCertificateSpec
	CACert           CaCertificateSpec
	Key              CertificateSpec
	Cert             CertificateSpec
	CN               string
	Options          []interface{}
}

type ManifestsSpec struct {
	GeneratedCertificates []GeneratedCertificateSpec
	Manifests             []ManifestSpec
}

func (m *ManifestsSpec) NeedChange() bool {
	for _, cert := range m.GeneratedCertificates {
		if cert.NeedChangeFileBy.NeedChange() {
			return true
		}
	}
	for _, manifest := range m.Manifests {
		if manifest.NeedChangeFileBy.NeedChange() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedStaticPodsCreate() bool {
	for _, manifest := range m.Manifests {
		if manifest.IsStaticPodManifest && manifest.NeedChangeFileBy.NeedCreate() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedStaticPodsUpdate() bool {
	for _, manifest := range m.Manifests {
		if manifest.IsStaticPodManifest && manifest.NeedChangeFileBy.NeedUpdate() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedManifestsCreate() bool {
	for _, manifest := range m.Manifests {
		if !manifest.IsStaticPodManifest && manifest.NeedChangeFileBy.NeedCreate() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedManifestsUpdate() bool {
	for _, manifest := range m.Manifests {
		if !manifest.IsStaticPodManifest && manifest.NeedChangeFileBy.NeedUpdate() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedStaticCertificatesCreate() bool {
	for _, cert := range m.GeneratedCertificates {
		if cert.NeedChangeFileBy.NeedCreate() {
			return true
		}
	}
	return false
}

func (m *ManifestsSpec) NeedStaticCertificatesUpdate() bool {
	for _, cert := range m.GeneratedCertificates {
		if cert.NeedChangeFileBy.NeedUpdate() {
			return true
		}
	}
	return false
}

func NewManifestsSpec() *ManifestsSpec {

	TmpWorkspaceDir := getTmpWorkspaceDir()
	TmpWorkspaceCertsDir := filepath.Join(TmpWorkspaceDir, "pki")
	TmpWorkspaceManifestsDir := filepath.Join(TmpWorkspaceDir, "manifests")
	TmpWorkspaceStaticPodsDir := filepath.Join(TmpWorkspaceManifestsDir, "static_pods")
	TmpWorkspaceSeaweedManifestsDir := filepath.Join(TmpWorkspaceManifestsDir, "seaweedfs_config")
	TmpWorkspaceDockerAuthManifestsDir := filepath.Join(TmpWorkspaceManifestsDir, "auth_config")
	TmpWorkspaceDockerDistribManifestsDir := filepath.Join(TmpWorkspaceManifestsDir, "distribution_config")

	InputCertsDir := getInputCertsDir()
	InputManifestsDir := getInputManifestsDir()
	InputStaticPodsDir := filepath.Join(InputManifestsDir, "static_pods")
	InputSeaweedManifestsDir := filepath.Join(InputManifestsDir, "seaweedfs_config")
	InputDockerAuthManifestsDir := filepath.Join(InputManifestsDir, "auth_config")
	InputDockerDistribManifestsDir := filepath.Join(InputManifestsDir, "distribution_config")

	DestionationDir := getDestionationDir()
	DestinationSystemRegistryDir := filepath.Join(DestionationDir, "system-registry")
	DestinationCertsDir := filepath.Join(DestinationSystemRegistryDir, "pki")
	DestionationDirStaticPodsDir := filepath.Join(DestionationDir, "manifests")
	DestionationSeaweedManifestsDir := filepath.Join(DestinationSystemRegistryDir, "seaweedfs_config")
	DestionationDockerAuthManifestsDir := filepath.Join(DestinationSystemRegistryDir, "auth_config")
	DestionationDockerDistribManifestsDir := filepath.Join(DestinationSystemRegistryDir, "distribution_config")

	cfg := GetConfig()

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

	manifestsSpec := ManifestsSpec{
		GeneratedCertificates: []GeneratedCertificateSpec{
			{
				CAKey:  baseCertificates.EtcdCAKey,
				CACert: baseCertificates.EtcdCACrt,
				Key: CertificateSpec{
					TmpGeneratePath: filepath.Join(TmpWorkspaceCertsDir, "seaweedfs-etcd-client.key"),
					DestPath:        filepath.Join(DestinationCertsDir, "seaweedfs-etcd-client.key"),
				},
				Cert: CertificateSpec{
					TmpGeneratePath: filepath.Join(TmpWorkspaceCertsDir, "seaweedfs-etcd-client.crt"),
					DestPath:        filepath.Join(DestinationCertsDir, "seaweedfs-etcd-client.crt"),
				},
				CN: "seaweedfs-etcd-client",
				Options: []interface{}{
					certificate.WithKeyAlgo("rsa"),
					certificate.WithKeySize(2048),
					certificate.WithGroups("system:masters"),
					certificate.WithSigningDefaultExpiry(365 * 24 * time.Hour),
					certificate.WithSigningDefaultUsage([]string{"digital signature", "key encipherment"}),
				},
			},
			{
				CAKey:  baseCertificates.EtcdCAKey,
				CACert: baseCertificates.EtcdCACrt,
				Key: CertificateSpec{
					TmpGeneratePath: filepath.Join(TmpWorkspaceCertsDir, "token.key"),
					DestPath:        filepath.Join(DestinationCertsDir, "token.key"),
				},
				Cert: CertificateSpec{
					TmpGeneratePath: filepath.Join(TmpWorkspaceCertsDir, "token.crt"),
					DestPath:        filepath.Join(DestinationCertsDir, "token.crt"),
				},
				CN: cfg.HostIP,
				Options: []interface{}{
					certificate.WithKeyAlgo("ecdsa"),
					certificate.WithKeySize(256),
					certificate.WithGroups("Deckhouse Registry"),
					certificate.WithSANs(cfg.HostIP),
				},
			},
		},
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
				InputPath:           filepath.Join(InputStaticPodsDir, "system-registry.yaml"),
				TmpPath:             filepath.Join(TmpWorkspaceStaticPodsDir, "system-registry.yaml"),
				DestPath:            filepath.Join(DestionationDirStaticPodsDir, "system-registry.yaml"),
				IsStaticPodManifest: true,
			},
		},
	}
	return &manifestsSpec
}

func NewManifestsSpecForTest() *ManifestsSpec {
	os.Setenv("IS_TEST", "true")
	return NewManifestsSpec()
}

func GetDataForManifestRendering() FileConfig {
	cfg := GetConfig()
	return cfg.FileConfig
}
