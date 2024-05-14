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

func UpdateManifests(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Starting UpdateManifests")

	if err := copyCertsToDest(manifestsSpec); err != nil {
		return err
	}
	if err := copyManifestsToDest(manifestsSpec); err != nil {
		return err
	}
	log.Info("UpdateManifests completed")
	return nil
}

func copyCertsToDest(manifestsSpec *config.ManifestsSpec) error {
	for _, cert := range manifestsSpec.GeneratedCertificates {
		if !cert.NeedChangeFileBy.NeedChange() {
			return nil
		}
		err := pkg.CopyFile(cert.Key.TmpGeneratePath, cert.Key.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert key from '%s' to '%s': %v", cert.Key.TmpGeneratePath, cert.Key.DestPath, err)
		}
		err = pkg.CopyFile(cert.Cert.TmpGeneratePath, cert.Cert.DestPath)
		if err != nil {
			return fmt.Errorf("error copying cert cert from '%s' to '%s': %v", cert.Cert.TmpGeneratePath, cert.Cert.DestPath, err)
		}
	}
	return nil
}

func copyManifestsToDest(manifestsSpec *config.ManifestsSpec) error {
	for _, manifest := range manifestsSpec.Manifests {
		if !manifest.NeedChangeFileBy.NeedChange() {
			return nil
		}
		err := pkg.CopyFile(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error copying manifests from '%s' to '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
	}
	return nil
}
