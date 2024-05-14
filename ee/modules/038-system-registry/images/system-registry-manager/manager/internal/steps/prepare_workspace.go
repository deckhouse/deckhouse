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

func PrepareWorkspace(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Starting workspace preparation...")

	if err := checkInputFilesExist(manifestsSpec); err != nil {
		log.Errorf("Error checking input files: %v", err)
		return err
	}
	if err := copyFilesToWorkspace(manifestsSpec); err != nil {
		log.Errorf("Error copying files to workspace: %v", err)
		return err
	}

	log.Info("Workspace preparation completed successfully.")
	return nil
}

func checkInputFilesExist(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Checking existence of input files...")

	inputFiles := []string{}

	for _, cert := range manifestsSpec.GeneratedCertificates {
		inputFiles = append(inputFiles, cert.CAKey.InputPath)
		inputFiles = append(inputFiles, cert.CACert.InputPath)
	}

	for _, manifest := range manifestsSpec.Manifests {
		inputFiles = append(inputFiles, manifest.InputPath)
	}

	for _, inputFile := range inputFiles {
		if !pkg.IsPathExists(inputFile) {
			return fmt.Errorf("can't find file '%s'", inputFile)
		}
	}

	log.Info("Input files check completed successfully.")
	return nil
}

func copyFilesToWorkspace(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Copying files to workspace...")

	for _, cert := range manifestsSpec.GeneratedCertificates {
		log.Infof("Copying CA key from %s to %s", cert.CAKey.InputPath, cert.CAKey.TmpPath)
		err := pkg.CopyFile(cert.CAKey.InputPath, cert.CAKey.TmpPath)
		if err != nil {
			return err
		}

		log.Infof("Copying CA certificate from %s to %s", cert.CACert.InputPath, cert.CACert.TmpPath)
		err = pkg.CopyFile(cert.CACert.InputPath, cert.CACert.TmpPath)
		if err != nil {
			return err
		}
	}

	renderData := config.GetDataForManifestRendering()
	for _, manifest := range manifestsSpec.Manifests {
		log.Infof("Copying manifest from %s to %s", manifest.InputPath, manifest.TmpPath)
		err := pkg.CopyFile(manifest.InputPath, manifest.TmpPath)
		if err != nil {
			return err
		}
		log.Infof("Rendering manifest template at %s", manifest.TmpPath)
		err = pkg.RenderTemplateFiles(manifest.TmpPath, renderData)
		if err != nil {
			return err
		}
	}

	log.Info("File copying to workspace completed successfully.")
	return nil
}
