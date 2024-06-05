/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"fmt"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"
	pkg_logs "system-registry-manager/pkg/logs"
)

func PrepareWorkspace(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Starting workspace preparation...")

	if err := checkInputCertificatesExist(ctx, manifestsSpec); err != nil {
		log.Errorf("Error checking input certificates: %v", err)
		return err
	}
	if err := checkInputManifestsExist(ctx, manifestsSpec); err != nil {
		log.Errorf("Error checking input manifests: %v", err)
		return err
	}
	if err := copyCertificatesToWorkspace(ctx, manifestsSpec); err != nil {
		log.Errorf("Error copying certificates to workspace: %v", err)
		return err
	}
	if err := copyManifestsToWorkspace(ctx, manifestsSpec); err != nil {
		log.Errorf("Error copying manifests to workspace: %v", err)
		return err
	}

	log.Debug("Workspace preparation completed successfully.")
	return nil
}

func checkInputCertificatesExist(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Checking existence of input certificates...")

	var inputFiles []string

	for _, cert := range manifestsSpec.GeneratedCertificates {
		inputFiles = append(inputFiles, cert.CAKey.InputPath)
		inputFiles = append(inputFiles, cert.CACert.InputPath)
	}

	for _, inputFile := range inputFiles {
		if !pkg_files.IsPathExists(inputFile) {
			return fmt.Errorf("can't find file '%s'", inputFile)
		}
	}

	log.Debug("Input certificates check completed successfully.")
	return nil
}

func checkInputManifestsExist(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Checking existence of input manifests...")

	for _, manifest := range manifestsSpec.Manifests {
		if !pkg_files.IsPathExists(manifest.InputPath) {
			return fmt.Errorf("can't find file '%s'", manifest.InputPath)
		}
	}

	log.Debug("Input manifests check completed successfully.")
	return nil
}

func copyCertificatesToWorkspace(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Copying certificates to workspace...")

	for _, cert := range manifestsSpec.GeneratedCertificates {
		log.Debugf("Copying CA key from %s to %s", cert.CAKey.InputPath, cert.CAKey.TmpPath)
		if err := pkg_files.CopyFile(cert.CAKey.InputPath, cert.CAKey.TmpPath); err != nil {
			return err
		}

		log.Debugf("Copying CA certificate from %s to %s", cert.CACert.InputPath, cert.CACert.TmpPath)
		if err := pkg_files.CopyFile(cert.CACert.InputPath, cert.CACert.TmpPath); err != nil {
			return err
		}
	}

	log.Debug("Certificate copying to workspace completed successfully.")
	return nil
}

func copyManifestsToWorkspace(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Copying manifests to workspace...")

	renderData, err := pkg_cfg.GetDataForManifestRendering()

	if err != nil {
		log.Fatalf("error decoding config: %v", err)
	}

	for _, manifest := range manifestsSpec.Manifests {
		log.Debugf("Copying manifest from %s to %s", manifest.InputPath, manifest.TmpPath)
		if err := pkg_files.CopyFile(manifest.InputPath, manifest.TmpPath); err != nil {
			return err
		}
		log.Debugf("Rendering manifest template at %s", manifest.TmpPath)
		if err := pkg_files.RenderTemplateFiles(manifest.TmpPath, renderData); err != nil {
			return err
		}
	}

	log.Debug("Manifest copying to workspace completed successfully.")
	return nil
}
