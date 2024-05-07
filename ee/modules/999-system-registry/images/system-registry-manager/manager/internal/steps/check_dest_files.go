/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"system-registry-manager/internal/config"
	"system-registry-manager/pkg"
)

func CheckDestFiles() error {
	if err := checkDestFileCheckSumAndExist(); err != nil {
		return err
	}
	validateSeaweedEtcdClientCert()
	validateDockerAuthTokenCert()
	return nil
}

func checkDestFileCheckSumAndExist() error {
	cfg := config.GetConfig()

	for _, manifest := range cfg.Manifests {
		if !pkg.IsPathExists(manifest.DestPath) {
			cfg.ShouldUpdateBy.NeedChangeFileByExist = true
			continue
		}
		isSumEq, err := pkg.CompareChecksum(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return err
		}
		if !isSumEq {
			cfg.ShouldUpdateBy.NeedChangeFileByExist = true
		}
	}
	return nil
}

func validateSeaweedEtcdClientCert() {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.ManifestsSpec.GeneratedCertificates.SeaweedEtcdClientCert.Cert.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.ManifestsSpec.GeneratedCertificates.SeaweedEtcdClientCert.Key.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}

func validateDockerAuthTokenCert() {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.ManifestsSpec.GeneratedCertificates.DockerAuthTokenCert.Cert.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.ManifestsSpec.GeneratedCertificates.DockerAuthTokenCert.Key.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}
