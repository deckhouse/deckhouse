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

func DeleteManifests(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Info("Starting delete step")

	if err := deleteCerts(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to delete certificates: %v", err)
		return err
	}
	if err := deleteManifests(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to delete manifests: %v", err)
		return err
	}
	log.Info("Delete step completed successfully")
	return nil
}

func deleteCerts(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Info("Starting to delete certificates")

	for _, cert := range manifestsSpec.GeneratedCertificates {

		log.Debugf("Deleting certificate key from '%s'", cert.Key.DestPath)
		err := pkg_files.DeleteFileIfExist(cert.Key.DestPath)
		if err != nil {
			return fmt.Errorf("error deleting cert key from '%s': %w", cert.Key.DestPath, err)
		}

		log.Debugf("Deleting certificate from '%s'", cert.Cert.DestPath)
		err = pkg_files.DeleteFileIfExist(cert.Cert.DestPath)
		if err != nil {
			return fmt.Errorf("error deleting cert from '%s': %w", cert.Cert.DestPath, err)
		}
	}

	log.Info("Certificates deleted successfully")
	return nil
}

func deleteManifests(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Info("Starting to delete manifests")

	for _, manifest := range manifestsSpec.Manifests {

		log.Debugf("Deleting manifest from '%s'", manifest.DestPath)
		err := pkg_files.DeleteFileIfExist(manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error deleting manifest from '%s': %w", manifest.DestPath, err)
		}
	}

	log.Info("Manifests deleted successfully")
	return nil
}
