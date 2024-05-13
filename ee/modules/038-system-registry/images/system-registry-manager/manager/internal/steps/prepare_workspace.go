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

func PrepareWorkspace() error {
	log.Info("Preparing workspace...")

	if err := checkInputFilesExist(); err != nil {
		log.Errorf("Error checking input files: %v", err)
		return err
	}
	if err := copyCertFilesToWorkspace(); err != nil {
		log.Errorf("Error copying cert files to workspace: %v", err)
		return err
	}
	if err := copyManifestFilesToWorkspace(); err != nil {
		log.Errorf("Error manifest cert files to workspace: %v", err)
		return err
	}
	log.Info("Workspace preparation completed.")
	return nil
}

func checkInputFilesExist() error {
	log.Info("Checking input files...")
	cfg := config.GetConfig()

	inputFiles := []string{
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.CAKey.InputPath,
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.CACert.InputPath,
		cfg.GeneratedCertificates.DockerAuthTokenCert.CAKey.InputPath,
		cfg.GeneratedCertificates.DockerAuthTokenCert.CACert.InputPath,
	}

	for _, manifest := range cfg.Manifests {
		inputFiles = append(inputFiles, manifest.InputPath)
	}

	for _, inputFile := range inputFiles {
		if !pkg.IsPathExists(inputFile) {
			return fmt.Errorf("can't find file '%s'", inputFile)
		}
	}
	log.Info("Input files check completed.")
	return nil
}

func copyCertFilesToWorkspace() error {
	log.Info("Copying cert files to workspace...")

	cfg := config.GetConfig()

	copyCertFiles := []FileMV{
		{
			From: cfg.GeneratedCertificates.SeaweedEtcdClientCert.CAKey.InputPath,
			To:   cfg.GeneratedCertificates.SeaweedEtcdClientCert.CAKey.TmpPath,
		},
		{
			From: cfg.GeneratedCertificates.SeaweedEtcdClientCert.CACert.InputPath,
			To:   cfg.GeneratedCertificates.SeaweedEtcdClientCert.CACert.TmpPath,
		},
		{
			From: cfg.GeneratedCertificates.DockerAuthTokenCert.CAKey.InputPath,
			To:   cfg.GeneratedCertificates.DockerAuthTokenCert.CAKey.TmpPath,
		},
		{
			From: cfg.GeneratedCertificates.DockerAuthTokenCert.CACert.InputPath,
			To:   cfg.GeneratedCertificates.DockerAuthTokenCert.CACert.TmpPath,
		},
	}
	for _, copyFile := range copyCertFiles {
		err := pkg.CopyFile(copyFile.From, copyFile.To)
		if err != nil {
			return err
		}
	}
	log.Info("Cert file copying to workspace completed.")
	return nil
}

func copyManifestFilesToWorkspace() error {
	log.Info("Copying manifest files to workspace...")

	cfg := config.GetConfig()
	renderData := config.GetDataForManifestRendering()

	for _, manifest := range cfg.Manifests {
		err := pkg.CopyFile(manifest.InputPath, manifest.TmpPath)
		if err != nil {
			return err
		}
		err = pkg.RenderTemplateFiles(manifest.TmpPath, renderData)
		if err != nil {
			return err
		}
	}
	log.Info("Manifest file copying to workspace completed.")
	return nil
}
