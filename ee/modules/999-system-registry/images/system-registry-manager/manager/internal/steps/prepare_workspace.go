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
	defer log.Info("Workspace preparation completed.")

	if err := checkInputFilesExist(); err != nil {
		log.Errorf("Error checking input files: %v", err)
		return err
	}
	if err := copyFilesToWorkspace(); err != nil {
		log.Errorf("Error copying files to workspace: %v", err)
		return err
	}
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

func copyFilesToWorkspace() error {
	log.Info("Copying files to workspace...")

	cfg := config.GetConfig()

	copyFiles := []FileMV{
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

	for _, manifest := range cfg.Manifests {
		copyFiles = append(
			copyFiles,
			FileMV{
				From: manifest.InputPath,
				To:   manifest.TmpPath,
			},
		)
	}

	for _, copyFile := range copyFiles {
		err := pkg.CopyFile(copyFile.From, copyFile.To)
		if err != nil {
			return err
		}
	}
	log.Info("File copying to workspace completed.")
	return nil
}
