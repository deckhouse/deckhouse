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

func CheckDestFiles(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Checking destination files...")

	if err := checkDestManifests(manifestsSpec); err != nil {
		log.Errorf("Error checking destination manifest files: %v", err)
		return err
	}

	if err := checkDestSerts(manifestsSpec); err != nil {
		log.Errorf("Error checking destination cert files: %v", err)
		return err
	}

	log.Info("Destination files check completed.")
	return nil
}

func checkDestManifests(manifestsSpec *config.ManifestsSpec) error {
	for i, manifest := range manifestsSpec.Manifests {
		if !pkg.IsPathExists(manifest.DestPath) {
			manifestsSpec.Manifests[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}
		isSumEq, err := pkg.CompareChecksum(manifest.TmpPath, manifest.DestPath)
		if err != nil {
			return fmt.Errorf("error comparing checksums for files '%s' and '%s': %v", manifest.TmpPath, manifest.DestPath, err)
		}
		if !isSumEq {
			manifestsSpec.Manifests[i].NeedChangeFileBy.NeedChangeFileByCheckSum = pkg.CreatePointer(true).(*bool)
		}
	}
	return nil
}

func checkDestSerts(manifestsSpec *config.ManifestsSpec) error {
	for i, cert := range manifestsSpec.GeneratedCertificates {
		if !pkg.IsPathExists(cert.Cert.DestPath) {
			manifestsSpec.GeneratedCertificates[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}
		if !pkg.IsPathExists(cert.Key.DestPath) {
			manifestsSpec.GeneratedCertificates[i].NeedChangeFileBy.NeedChangeFileByExist = true
			continue
		}
		// TODO
	}
	return nil
}
