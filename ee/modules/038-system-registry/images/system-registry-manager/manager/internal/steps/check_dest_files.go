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

func CheckDestFiles(shouldUpdateBy *ShouldUpdateBy) error {
	log.Info("Checking destination files...")

	if err := checkDestFileCheckSumAndExist(shouldUpdateBy); err != nil {
		log.Errorf("Error checking destination files: %v", err)
		return err
	}
	validateSeaweedEtcdClientCert(shouldUpdateBy)
	validateDockerAuthTokenCert(shouldUpdateBy)

	log.Info("Destination files check completed.")
	return nil
}

func checkDestFileCheckSumAndExist(shouldUpdateBy *ShouldUpdateBy) error {
	cfg := config.GetConfig()

	for _, manifest := range cfg.Manifests {
		if !pkg.IsPathExists(manifest.DestPath) {
			shouldUpdateBy.NeedChangeFileByExist = true
			continue
		}
		isSumEq, err := pkg.CompareChecksum(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error comparing checksums for files '%s' and '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
		if !isSumEq {
			shouldUpdateBy.NeedChangeFileByExist = true
		}
	}
	return nil
}

func validateSeaweedEtcdClientCert(shouldUpdateBy *ShouldUpdateBy) {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.GeneratedCertificates.SeaweedEtcdClientCert.Cert.DestPath) {
		shouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.GeneratedCertificates.SeaweedEtcdClientCert.Key.DestPath) {
		shouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}

func validateDockerAuthTokenCert(shouldUpdateBy *ShouldUpdateBy) {
	cfg := config.GetConfig()

	if !pkg.IsPathExists(cfg.GeneratedCertificates.DockerAuthTokenCert.Cert.DestPath) {
		shouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
	if !pkg.IsPathExists(cfg.GeneratedCertificates.DockerAuthTokenCert.Key.DestPath) {
		shouldUpdateBy.NeedChangeSeaweedfsCerts = true
		return
	}
}
