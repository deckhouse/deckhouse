/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cfg

import (
	"os"
	"path/filepath"
	"time"

	"fmt"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/certificate"
	"github.com/mitchellh/mapstructure"
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
	InputPath string
	DestPath  string
}
type StaticPodManifestSpec struct {
	InputPath string
	DestPath  string
}

type CaCertificateSpec struct {
	InputPath string
}

type CertificateSpec struct {
	DestPath string
}

type BaseCertificatesSpec struct {
	CACrt     CaCertificateSpec
	CAKey     CaCertificateSpec
	EtcdCACrt CaCertificateSpec
	EtcdCAKey CaCertificateSpec
}

type GeneratedCertificateSpec struct {
	CAKey   CaCertificateSpec
	CACert  CaCertificateSpec
	Key     CertificateSpec
	Cert    CertificateSpec
	CN      string
	Options []interface{}
}

type ManifestsSpec struct {
	GeneratedCertificates []GeneratedCertificateSpec
	Manifests             []ManifestSpec
	StaticPods            []StaticPodManifestSpec
}

func NewManifestsSpec() *ManifestsSpec {
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
		},
		CAKey: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "ca.key"),
		},
		EtcdCACrt: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "etcd-ca.crt"),
		},
		EtcdCAKey: CaCertificateSpec{
			InputPath: filepath.Join(InputCertsDir, "etcd-ca.key"),
		},
	}

	manifestsSpec := ManifestsSpec{
		GeneratedCertificates: []GeneratedCertificateSpec{
			{
				CAKey:  baseCertificates.EtcdCAKey,
				CACert: baseCertificates.EtcdCACrt,
				Key: CertificateSpec{
					DestPath: filepath.Join(DestinationCertsDir, "seaweedfs-etcd-client.key"),
				},
				Cert: CertificateSpec{
					DestPath: filepath.Join(DestinationCertsDir, "seaweedfs-etcd-client.crt"),
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
					DestPath: filepath.Join(DestinationCertsDir, "token.key"),
				},
				Cert: CertificateSpec{
					DestPath: filepath.Join(DestinationCertsDir, "token.crt"),
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
				InputPath: filepath.Join(InputDockerDistribManifestsDir, "config.yaml.tpl"),
				DestPath:  filepath.Join(DestionationDockerDistribManifestsDir, "config.yaml"),
			},
			{
				InputPath: filepath.Join(InputDockerAuthManifestsDir, "config.yaml.tpl"),
				DestPath:  filepath.Join(DestionationDockerAuthManifestsDir, "config.yaml"),
			},
			{
				InputPath: filepath.Join(InputSeaweedManifestsDir, "filer.toml.tpl"),
				DestPath:  filepath.Join(DestionationSeaweedManifestsDir, "filer.toml"),
			},
			{
				InputPath: filepath.Join(InputSeaweedManifestsDir, "master.toml.tpl"),
				DestPath:  filepath.Join(DestionationSeaweedManifestsDir, "master.toml"),
			},
		},
		StaticPods: []StaticPodManifestSpec{
			{
				InputPath: filepath.Join(InputStaticPodsDir, "system-registry.yaml.tpl"),
				DestPath:  filepath.Join(DestionationDirStaticPodsDir, "system-registry.yaml"),
			},
		},
	}
	return &manifestsSpec
}

func NewManifestsSpecForTest() *ManifestsSpec {
	os.Setenv("IS_TEST", "true")
	return NewManifestsSpec()
}

func NewExtraDataForManifestRendering(masterPeers []string) *ExtraDataForManifestRendering {
	return &ExtraDataForManifestRendering{
		MasterPeers: masterPeers,
	}
}

type ExtraDataForManifestRendering struct {
	MasterPeers []string `mapstructure:"masterPeers"`
}

func (ext *ExtraDataForManifestRendering) DecodeToMapstructure() (map[string]interface{}, error) {
	var configMap map[string]interface{}

	err := mapstructure.Decode(ext, &configMap)
	if err != nil {
		return nil, fmt.Errorf("error decoding config: %v", err)
	}
	return configMap, nil
}

func GetDataForManifestRendering(extData *ExtraDataForManifestRendering) (map[string]interface{}, error) {
	dataMapStruct, err := (GetConfig().FileConfig).DecodeToMapstructure()
	if err != nil {
		return nil, err
	}
	if extData != nil {
		extDataMapStruct, err := extData.DecodeToMapstructure()
		if err != nil {
			return nil, err
		}
		for key, value := range extDataMapStruct {
			dataMapStruct[key] = value
		}
	}
	return dataMapStruct, err
}
