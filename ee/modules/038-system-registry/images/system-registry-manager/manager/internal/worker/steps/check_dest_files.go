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

func CheckDestFiles(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Starting check of destination files...")

	if err := checkDestManifests(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to check destination manifest files: %v", err)
		return err
	}

	if err := checkDestSerts(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to check destination certificate files: %v", err)
		return err
	}

	log.Debug("Destination files check completed successfully.")
	return nil
}

func checkDestManifests(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Checking destination manifest files...")

	for i, manifest := range manifestsSpec.Manifests {
		if !pkg_files.IsPathExists(manifest.DestPath) {
			log.Warnf("Destination path does not exist for manifest: %s", manifest.DestPath)
			manifestsSpec.Manifests[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}

		isSumEq, err := pkg_files.CompareChecksum(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error comparing checksums for files '%s' and '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}

		if !isSumEq {
			log.Warnf("Checksum mismatch for manifest: %s", manifest.DestPath)
			NeedChangeFileByCheckSum := true
			manifestsSpec.Manifests[i].NeedChangeFileBy.NeedChangeFileByCheckSum = &NeedChangeFileByCheckSum
		}
	}
	log.Debug("Completed checking destination manifest files.")
	return nil
}

func checkDestSerts(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Checking destination certificate files...")

	for i, cert := range manifestsSpec.GeneratedCertificates {
		if !pkg_files.IsPathExists(cert.Cert.DestPath) {
			log.Warnf("Destination certificate path does not exist: %s", cert.Cert.DestPath)
			manifestsSpec.GeneratedCertificates[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}

		if !pkg_files.IsPathExists(cert.Key.DestPath) {
			log.Warnf("Destination key path does not exist: %s", cert.Key.DestPath)
			manifestsSpec.GeneratedCertificates[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}

		// Additional checks can be added here.
	}
	log.Debug("Completed checking destination certificate files.")
	return nil
}
