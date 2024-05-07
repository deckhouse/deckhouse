/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"system-registry-manager/internal/config"
	"system-registry-manager/pkg"
)

func UpdateManifests() error {
	if err := copyCertsToDets(); err != nil {
		return err
	}
	if err := copyManifestsToDets(); err != nil {
		return err
	}
	return nil
}

func copyCertsToDets() error {
	cfg := config.GetConfig()
	copyFiles := []FileMV{}

	if cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts {
		copyFiles = append(copyFiles,
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

	if cfg.ShouldUpdateBy.NeedChangeDockerAuthTokenCerts {
		copyFiles = append(copyFiles,
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

	for _, copyFile := range copyFiles {
		err := pkg.CopyFile(copyFile.From, copyFile.To)
		if err != nil {
			return err
		}
	}
	return nil
}

func copyManifestsToDets() error {
	cfg := config.GetConfig()

	if !(cfg.ShouldUpdateBy.NeedChangeFileByCheckSum || cfg.ShouldUpdateBy.NeedChangeFileByExist) {
		return nil
	}

	for _, manifest := range cfg.Manifests {
		err := pkg.CopyFile(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return err
		}
	}
	return nil
}
