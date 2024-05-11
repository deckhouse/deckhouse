/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"system-registry-manager/internal/config"
	"system-registry-manager/pkg"
)

func CheckDestFiles() error {
	log.Info("Checking destination files...")

	if err := checkDestFileCheckSumAndExist(); err != nil {
		log.Errorf("Error checking destination files: %v", err)
		return err
	}
	validateSeaweedEtcdClientCert()
	validateDockerAuthTokenCert()

	log.Info("Destination files check completed.")
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
			return fmt.Errorf("error comparing checksums for files '%s' and '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
		if !isSumEq {
			cfg.ShouldUpdateBy.NeedChangeFileByExist = true
		}
	}
	return nil
}

func validateSeaweedEtcdClientCert() {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.GeneratedCertificates.SeaweedEtcdClientCert.Cert.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.GeneratedCertificates.SeaweedEtcdClientCert.Key.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}

func validateDockerAuthTokenCert() {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.GeneratedCertificates.DockerAuthTokenCert.Cert.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.GeneratedCertificates.DockerAuthTokenCert.Key.DestPath) {
		cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}
