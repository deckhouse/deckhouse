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

func UpdateManifests(shouldUpdateBy *ShouldUpdateBy) error {
	log.Info("Starting UpdateManifests")

	if err := copyCertsToDets(shouldUpdateBy); err != nil {
		return err
	}
	if err := copyManifestsToDets(shouldUpdateBy); err != nil {
		return err
	}
	log.Info("UpdateManifests completed")
	return nil
}

func copyCertsToDets(shouldUpdateBy *ShouldUpdateBy) error {
	cfg := config.GetConfig()
	copyFilesCerts := []FileMV{}

	if shouldUpdateBy.NeedChangeSeaweedfsCerts {
		copyFilesCerts = append(copyFilesCerts,
			[]FileMV{
				{
					From: cfg.GeneratedCertificates.SeaweedEtcdClientCert.Cert.TmpGeneratePath,
					To:   cfg.GeneratedCertificates.SeaweedEtcdClientCert.Cert.DestPath,
				},
				{
					From: cfg.GeneratedCertificates.SeaweedEtcdClientCert.Key.TmpGeneratePath,
					To:   cfg.GeneratedCertificates.SeaweedEtcdClientCert.Key.DestPath,
				},
			}...,
		)
	}

	if shouldUpdateBy.NeedChangeDockerAuthTokenCerts {
		copyFilesCerts = append(copyFilesCerts,
			[]FileMV{
				{
					From: cfg.GeneratedCertificates.DockerAuthTokenCert.Cert.TmpGeneratePath,
					To:   cfg.GeneratedCertificates.DockerAuthTokenCert.Cert.DestPath,
				},
				{
					From: cfg.GeneratedCertificates.DockerAuthTokenCert.Key.TmpGeneratePath,
					To:   cfg.GeneratedCertificates.DockerAuthTokenCert.Key.DestPath,
				},
			}...,
		)
	}

	for _, copyFile := range copyFilesCerts {
		err := pkg.CopyFile(copyFile.From, copyFile.To)
		if err != nil {
			return fmt.Errorf("error copying cert from '%s' to '%s': %v", copyFile.From, copyFile.To, err)
		}
	}
	return nil
}

func copyManifestsToDets(shouldUpdateBy *ShouldUpdateBy) error {
	cfg := config.GetConfig()

	if !(shouldUpdateBy.NeedChangeFileByCheckSum || shouldUpdateBy.NeedChangeFileByExist) {
		return nil
	}

	for _, manifest := range cfg.Manifests {
		err := pkg.CopyFile(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error copying manifests from '%s' to '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
	}
	return nil
}
