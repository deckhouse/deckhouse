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

func UpdateManifests(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Starting UpdateManifests")

	if err := copyCertsToDest(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to copy certificates: %v", err)
		return err
	}
	if err := copyManifestsToDest(ctx, manifestsSpec); err != nil {
		log.Errorf("Failed to copy manifests: %v", err)
		return err
	}
	log.Debug("UpdateManifests completed successfully")
	return nil
}

func copyCertsToDest(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Starting to copy certificates to destination")

	for _, cert := range manifestsSpec.GeneratedCertificates {
		if !cert.NeedChangeFileBy.NeedChange() {
			log.Debugf("No changes needed for certificate: %s", cert.Cert.DestPath)
			continue
		}

		log.Debugf("Copying certificate key from '%s' to '%s'", cert.Key.TmpGeneratePath, cert.Key.DestPath)
		err := pkg_files.CopyFile(cert.Key.TmpGeneratePath, cert.Key.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert key from '%s' to '%s': %v", cert.Key.TmpGeneratePath, cert.Key.DestPath, err)
		}

		log.Debugf("Copying certificate from '%s' to '%s'", cert.Cert.TmpGeneratePath, cert.Cert.DestPath)
		err = pkg_files.CopyFile(cert.Cert.TmpGeneratePath, cert.Cert.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert from '%s' to '%s': %v", cert.Cert.TmpGeneratePath, cert.Cert.DestPath, err)
		}
	}

	log.Debug("Certificates copied to destination successfully")
	return nil
}

func copyManifestsToDest(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log := pkg_logs.GetLoggerFromContext(ctx)
	log.Debug("Starting to copy manifests to destination")

	for _, manifest := range manifestsSpec.Manifests {
		if !manifest.NeedChangeFileBy.NeedChange() {
			log.Debugf("No changes needed for manifest: %s", manifest.DestPath)
			continue
		}

		log.Debugf("Copying manifest from '%s' to '%s'", manifest.TmpPath, manifest.DestPath)
		err := pkg_files.CopyFile(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error copying manifest from '%s' to '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
	}

	log.Debug("Manifests copied to destination successfully")
	return nil
}
