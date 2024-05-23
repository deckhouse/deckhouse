/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"
)

func UpdateManifests(manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log.Info("Starting UpdateManifests")

	if err := copyCertsToDest(manifestsSpec); err != nil {
		log.Errorf("Failed to copy certificates: %v", err)
		return err
	}
	if err := copyManifestsToDest(manifestsSpec); err != nil {
		log.Errorf("Failed to copy manifests: %v", err)
		return err
	}
	log.Info("UpdateManifests completed successfully")
	return nil
}

func copyCertsToDest(manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log.Info("Starting to copy certificates to destination")

	for _, cert := range manifestsSpec.GeneratedCertificates {
		if !cert.NeedChangeFileBy.NeedChange() {
			log.Infof("No changes needed for certificate: %s", cert.Cert.DestPath)
			continue
		}

		log.Infof("Copying certificate key from '%s' to '%s'", cert.Key.TmpGeneratePath, cert.Key.DestPath)
		err := pkg_files.CopyFile(cert.Key.TmpGeneratePath, cert.Key.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert key from '%s' to '%s': %v", cert.Key.TmpGeneratePath, cert.Key.DestPath, err)
		}

		log.Infof("Copying certificate from '%s' to '%s'", cert.Cert.TmpGeneratePath, cert.Cert.DestPath)
		err = pkg_files.CopyFile(cert.Cert.TmpGeneratePath, cert.Cert.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert from '%s' to '%s': %v", cert.Cert.TmpGeneratePath, cert.Cert.DestPath, err)
		}
	}

	log.Info("Certificates copied to destination successfully")
	return nil
}

func copyManifestsToDest(manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log.Info("Starting to copy manifests to destination")

	for _, manifest := range manifestsSpec.Manifests {
		if !manifest.NeedChangeFileBy.NeedChange() {
			log.Infof("No changes needed for manifest: %s", manifest.DestPath)
			continue
		}

		log.Infof("Copying manifest from '%s' to '%s'", manifest.TmpPath, manifest.DestPath)
		err := pkg_files.CopyFile(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error copying manifest from '%s' to '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
	}

	log.Info("Manifests copied to destination successfully")
	return nil
}
